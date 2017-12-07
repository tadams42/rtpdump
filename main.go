package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/hdiniz/rtpdump/codecs"
	"github.com/hdiniz/rtpdump/console"
	"github.com/hdiniz/rtpdump/esp"
	"github.com/hdiniz/rtpdump/log"
	"github.com/hdiniz/rtpdump/rtp"
	"github.com/urfave/cli"
)

func loadKeyFile(c *cli.Context) error {
	return esp.LoadKeyFile(c.GlobalString("key-file"))
}

var streamsCmd = func(c *cli.Context) error {
	loadKeyFile(c)

	inputFile := c.Args().First()

	if len(c.Args()) <= 0 {
		cli.ShowCommandHelp(c, "streams")
		return cli.NewExitError("wrong usage for streams", 1)
	}

	rtpReader, err := rtp.NewRtpReader(inputFile)

	if err != nil {
		return cli.NewMultiError(cli.NewExitError("failed to open file", 1), err)
	}

	defer rtpReader.Close()

	rtpStreams := rtpReader.GetStreams()

	if len(rtpStreams) <= 0 {
		fmt.Println("No streams found")
		return nil
	}

	for _, v := range rtpStreams {
		fmt.Printf("%s\n", v)
	}

	return nil
}

var playCmd = func(c *cli.Context) error {

	loadKeyFile(c)

	inputFile := c.Args().First()

	if inputFile == "" {
		cli.ShowCommandHelp(c, "play")
		return cli.NewExitError("wrong usage for play", 1)
	}

	host := c.String("host")
	port := c.Int("port")

	rtpReader, err := rtp.NewRtpReader(inputFile)

	if err != nil {
		return cli.NewMultiError(cli.NewExitError("failed to open file", 1), err)
	}

	defer rtpReader.Close()

	rtpStreams := rtpReader.GetStreams()

	if len(rtpStreams) <= 0 {
		fmt.Println("No streams found")
		return nil
	}

	var rtpStreamsOptions []string
	for _, v := range rtpStreams {
		rtpStreamsOptions = append(rtpStreamsOptions, v.String())
	}

	streamIndex, err := console.ExpectIntRange(
		1,
		len(rtpStreams),
		console.ListPrompt("Choose RTP Stream", rtpStreamsOptions...))

	if err != nil {
		return cli.NewMultiError(cli.NewExitError("invalid input", 1), err)
	}
	fmt.Printf("(%-3d) %s\n\n", streamIndex, rtpStreams[streamIndex-1])

	stream := rtpStreams[streamIndex-1]

	fmt.Println(stream)

	RemoteAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	conn, err := net.DialUDP("udp", nil, RemoteAddr)
	defer conn.Close()
	if err != nil {
		fmt.Printf("Some error %v", err)
		return nil
	}

	len := len(stream.RtpPackets)
	for i, v := range stream.RtpPackets {
		fmt.Println(v)
		conn.Write(v.Data)

		if i < len-1 {
			var wait int
			next := stream.RtpPackets[i+1]
			wait = next.ReceivedAt.Nanosecond() - v.ReceivedAt.Nanosecond()
			time.Sleep(time.Nanosecond * time.Duration(wait))
		}
	}

	return nil
}

var dumpCmd = func(c *cli.Context) error {

	loadKeyFile(c)

	inputFile := c.Args().First()

	if inputFile == "" {
		cli.ShowCommandHelp(c, "dump")
		return cli.NewExitError("wrong usage for dump", 1)
	}

	rtpReader, err := rtp.NewRtpReader(inputFile)

	if err != nil {
		return cli.NewMultiError(cli.NewExitError("failed to open file", 1), err)
	}

	defer rtpReader.Close()

	return doInteractiveDump(c, rtpReader)
}

func doInteractiveDump(c *cli.Context, rtpReader *rtp.RtpReader) error {
	rtpStreams := rtpReader.GetStreams()

	if len(rtpStreams) <= 0 {
		fmt.Println("No streams found")
		return nil
	}

	var rtpStreamsOptions []string
	for _, v := range rtpStreams {
		rtpStreamsOptions = append(rtpStreamsOptions, v.String())
	}

	streamIndex, err := console.ExpectIntRange(
		1,
		len(rtpStreams),
		console.ListPrompt("Choose RTP Stream", rtpStreamsOptions...))

	if err != nil {
		return cli.NewMultiError(cli.NewExitError("invalid input", 1), err)
	}
	fmt.Printf("(%-3d) %s\n\n", streamIndex, rtpStreams[streamIndex-1])

	var codecList []string
	for _, v := range codecs.CodecList {
		codecList = append(codecList, v.Name)
	}
	codecIndex, err := console.ExpectIntRange(
		1,
		len(codecs.CodecList),
		console.ListPrompt("Choose codec:", codecList...))

	if err != nil {
		return cli.NewMultiError(cli.NewExitError("invalid input", 1), err)
	}
	fmt.Printf("(%-3d) %s\n\n", codecIndex, codecs.CodecList[codecIndex-1].Name)

	codecMetadata := codecs.CodecList[codecIndex-1]

	optionsMap := make(map[string]string)
	for _, v := range codecMetadata.Options {
		var optionValue string
		if v.RestrictValues {
			optionValue, err = console.ExpectRestrictedString(
				v.ValidValues,
				console.KeyValuePrompt(fmt.Sprintf("%s - %s", v.Name, v.Description),
					v.ValidValues, v.ValueDescription))
		} else {
			optionValue, err = console.ExpectAnyString(
				console.Prompt(fmt.Sprintf("%s - %s: ", v.Name, v.Description)))
		}

		if err != nil {
			return cli.NewMultiError(cli.NewExitError("invalid input", 1), err)
		}
		optionsMap[v.Name] = optionValue
	}

	outputFile, err := console.ExpectAnyString(console.Prompt("Output file: "))

	if err != nil {
		return cli.NewMultiError(cli.NewExitError("invalid input", 1), err)
	}

	fmt.Printf("%s\n", outputFile)

	codec := codecMetadata.Init()
	err = codec.SetOptions(optionsMap)

	if err != nil {
		return err
	}

	codec.Init()

	f, err := os.Create(outputFile)
	defer f.Close()
	f.Write(codec.GetFormatMagic())
	for _, r := range rtpStreams[streamIndex-1].RtpPackets {
		frames, err := codec.HandleRtpPacket(r)
		if err == nil {
			f.Write(frames)
		}
	}
	f.Sync()

	return nil
}

func codecsList(c *cli.Context) error {
	codec := c.Args().First()
	found := codec == ""

	for _, v := range codecs.CodecList {
		if found || codec == v.Name {
			fmt.Printf("%s\n", v.Describe())
			found = true
		}
	}

	if !found {
		fmt.Printf("Codec %s not available\n", codec)
	}
	return nil
}

var dumpAllAmrCmd = func(c *cli.Context) error {

	loadKeyFile(c)

	inputFile := c.Args().First()
	if inputFile == "" {
		cli.ShowCommandHelp(c, "dump_amr")
		return cli.NewExitError("No input pcap given!", 1)
	}

	outputPath := c.String("output-path")
	if outputPath == "" {
		cli.ShowCommandHelp(c, "dump_amr")
		return cli.NewExitError("No output path given!", 1)
	}

	streamIndex := c.Int("stream-index")
	if streamIndex < 0 {
		cli.ShowCommandHelp(c, "dump_amr")
		return cli.NewExitError("You must speciffy stream index!", 1)
	}

	wideband := c.Bool("wideband")
	octetAligned := c.Bool("octet-aligned")

	rtpReader, err := rtp.NewRtpReader(inputFile)
	if err != nil {
		return cli.NewMultiError(cli.NewExitError("failed to open file", 1), err)
	}
	defer rtpReader.Close()

	rtpStreams := rtpReader.GetStreams()
	if len(rtpStreams) <= 0 {
		return cli.NewExitError("No streams found!", 1)
	}
	if streamIndex >= len(rtpStreams) {
		return cli.NewExitError("No stream with such index!", 1)
	}

	stream := rtpStreams[streamIndex]
	// fmt.Println(stream)

	codecMetadata := codecs.CodecList[0]
	for i := range codecs.CodecList {
		if codecs.CodecList[i].Name == "amr" {
			codecMetadata = codecs.CodecList[i]
		}
	}
	// fmt.Println(codecMetadata)

	optionsMap := make(map[string]string)
	if wideband {
		optionsMap["sample-rate"] = "wb"
	} else {
		optionsMap["sample-rate"] = "nb"
	}
	if octetAligned {
		optionsMap["octet-aligned"] = "1"
	} else {
		optionsMap["octet-aligned"] = "0"
	}
	// fmt.Println(optionsMap)

	codec := codecMetadata.Init()
	err = codec.SetOptions(optionsMap)
	if err != nil {
		return err
	}

	codec.Init()
	// fmt.Println(codec)

	f, err := os.Create(outputPath)
	defer f.Close()
	f.Write(codec.GetFormatMagic())
	for _, r := range stream.RtpPackets {
		frames, err := codec.HandleRtpPacket(r)
		if err == nil {
			f.Write(frames)
		}
	}
	f.Sync()

	return nil
}

func main() {

	log.SetLevel(log.INFO)

	app := cli.NewApp()
	app.Name = "rtpdump"
	app.Version = "0.9.0"
	cli.AppHelpTemplate += `
     /\_/\
    ( o.o )
     > ^ <
    `

	app.Commands = []cli.Command{
		{
			Name:      "streams",
			Aliases:   []string{"s"},
			Usage:     "display rtp streams in pcap file",
			ArgsUsage: "[pcap-file]",
			Action:    streamsCmd,
		},
		{
			Name:      "dump",
			Aliases:   []string{"d"},
			Usage:     "dumps rtp payload to file",
			ArgsUsage: "[pcap-file]",
			Action:    dumpCmd,
		},
		{
			Name:      "play",
			Aliases:   []string{"p"},
			Usage:     "replays the selected rtp stream ;)",
			ArgsUsage: "[pcap-file]",
			Action:    playCmd,
			Flags: []cli.Flag{
				cli.StringFlag{Name: "host", Value: "localhost", Usage: "destination host for replayed RTP packets"},
				cli.IntFlag{Name: "port", Value: 1234, Usage: "destination port for replayed RTP packets"},
			},
		},
		{
			Name:      "dump_amr",
			Aliases:   []string{"a"},
			Usage:     "dumps all streams from pcap file, assuming AMR/AMR-WB codec",
			ArgsUsage: "[pcap-file]",
			Action:    dumpAllAmrCmd,
			Flags: []cli.Flag{
				cli.IntFlag{Name: "stream-index, s", Value: -1, Usage: "Index of stream (zero based) that will be dumped. Use 'streams' to list them"},
				cli.BoolFlag{Name: "wideband, w", Usage: "Is AMR-WB wideband? Omitting this flag means it is narrowband"},
				cli.BoolFlag{Name: "octet-aligned, a", Usage: "Is AMR octet-aligned? Omitting this flag means it is bandwidth-efficient"},
				cli.StringFlag{Name: "output-path, o", Usage: "Path to output file."},
			},
		},
		{
			Name:    "codecs",
			Aliases: []string{"c"},
			Usage:   "lists supported codecs information",
			Subcommands: cli.Commands{
				cli.Command{
					Name:      "list",
					Action:    codecsList,
					ArgsUsage: "[codec name or empty for all]",
				},
			},
		},
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "key-file, k",
			Value: "esp-keys.txt",
			Usage: "Load ipsec keys from `FILE`",
		},
	}

	app.Run(os.Args)
}
