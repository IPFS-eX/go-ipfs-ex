package plugin

import (
	"github.com/IPFS-eX/go-ipfs-ex/core/coredag"

	ipld "github.com/ipfs/go-ipld-format"
)

// PluginIPLD is an interface that can be implemented to add handlers for
// for different IPLD formats
type PluginIPLD interface {
	Plugin

	RegisterBlockDecoders(dec ipld.BlockDecoder) error
	RegisterInputEncParsers(iec coredag.InputEncParsers) error
}
