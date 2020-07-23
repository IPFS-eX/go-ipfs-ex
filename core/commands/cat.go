package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/IPFS-eX/go-ipfs-ex/core/commands/cmdenv"
	"github.com/IPFS-eX/go-ipfs-ex/core/coreapi"
	"github.com/IPFS-eX/go-ipfs-ex/core/crypto"

	iface "github.com/IPFS-eX/interface-go-ipfs-core"
	"github.com/IPFS-eX/interface-go-ipfs-core/path"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
)

const (
	progressBarMinSize   = 1024 * 1024 * 8 // show progress bar for outputs > 8MiB
	offsetOptionName     = "offset"
	lengthOptionName     = "length"
	decryptPwdOptionName = "decryptPassword"
)

var CatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Show IPFS object data.",
		ShortDescription: "Displays the data contained by an IPFS or IPNS object(s) at the given path.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to be outputted.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.Int64Option(offsetOptionName, "o", "Byte offset to begin reading from."),
		cmds.Int64Option(lengthOptionName, "l", "Maximum number of bytes to read."),
		cmds.StringOption(decryptPwdOptionName, "Decrypt password to decrypt the file").WithDefault(""),
		cmds.BoolOption(publishOptionName, "Publish file information to DNS network").WithDefault(true),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		offset, _ := req.Options[offsetOptionName].(int64)
		if offset < 0 {
			return fmt.Errorf("cannot specify negative offset")
		}

		max, found := req.Options[lengthOptionName].(int64)
		publish, _ := req.Options[publishOptionName].(bool)
		if max < 0 {
			return fmt.Errorf("cannot specify negative length")
		}
		if !found {
			max = -1
		}
		err = req.ParseBodyArgs()
		if err != nil {
			return err
		}
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}
		fileName := ""
		if len(req.Arguments) > 0 {
			cfgM, _ := readConfig(env)
			api.Scan().Startup(n.PrivateKey, cfgM)
			for _, hash := range req.Arguments {
				fi, err := api.Scan().GetFilePeers(req.Context, hash)
				if err != nil {
					break
				}
				fileName = fi.GetName()
				pis, err := parseAddresses(req.Context, fi.GetPeers())
				if err != nil {
					break
				}
				for _, p := range pis {
					go api.Swarm().Connect(req.Context, p)
				}
			}
		}

		readers, length, err := cat(req.Context, api, req.Arguments, int64(offset), int64(max))
		if err != nil {
			return err
		}

		/*
			if err := corerepo.ConditionalGC(req.Context, node, length); err != nil {
				re.SetError(err, cmds.ErrNormal)
				return
			}
		*/
		decryptPwdStr, _ := req.Options[decryptPwdOptionName].(string)
		res.SetLength(length)
		if len(decryptPwdStr) > 0 {
			newReaders := make([]io.Reader, 0, len(readers))
			for _, reader := range readers {
				reader, err = crypto.AESDecryptFileReader(reader, decryptPwdStr)
				if err != nil {
					return err
				}
				newReaders = append(newReaders, reader)
			}
			readers = newReaders
		}

		if offset == 0 && max == -1 && publish {
			for _, hash := range req.Arguments {
				api.Scan().PublishFile(req.Context, &coreapi.ScanFileInfo{
					Name: fileName,
					Hash: hash,
				}, n.PeerHost)
			}
		}
		reader := io.MultiReader(readers...)
		// Since the reader returns the error that a block is missing, and that error is
		// returned from io.Copy inside Emit, we need to take Emit errors and send
		// them to the client. Usually we don't do that because it means the connection
		// is broken or we supplied an illegal argument etc.
		return res.Emit(reader)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			if res.Length() > 0 && res.Length() < progressBarMinSize {
				return cmds.Copy(re, res)
			}

			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}

				switch val := v.(type) {
				case io.Reader:
					bar, reader := progressBarForReader(os.Stderr, val, int64(res.Length()))
					bar.Start()

					err = re.Emit(reader)
					if err != nil {
						return err
					}
				default:
					log.Warnf("cat postrun: received unexpected type %T", val)
				}
			}
		},
	},
}

func cat(ctx context.Context, api iface.CoreAPI, paths []string, offset int64, max int64) ([]io.Reader, uint64, error) {
	readers := make([]io.Reader, 0, len(paths))
	length := uint64(0)
	if max == 0 {
		return nil, 0, nil
	}
	for _, p := range paths {
		f, err := api.Unixfs().Get(ctx, path.New(p))
		if err != nil {
			return nil, 0, err
		}

		var file files.File
		switch f := f.(type) {
		case files.File:
			file = f
		case files.Directory:
			return nil, 0, iface.ErrIsDir
		default:
			return nil, 0, iface.ErrNotSupported
		}

		fsize, err := file.Size()
		if err != nil {
			return nil, 0, err
		}

		if offset > fsize {
			offset = offset - fsize
			continue
		}

		count, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			return nil, 0, err
		}
		offset = 0

		fsize, err = file.Size()
		if err != nil {
			return nil, 0, err
		}

		size := uint64(fsize - count)
		length += size
		if max > 0 && length >= uint64(max) {
			var r io.Reader = file
			if overshoot := int64(length - uint64(max)); overshoot != 0 {
				r = io.LimitReader(file, int64(size)-overshoot)
				length = uint64(max)
			}
			readers = append(readers, r)
			break
		}
		readers = append(readers, file)
	}
	return readers, length, nil
}
