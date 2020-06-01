package loader

import (
	pluginbadgerds "github.com/IPFS-eX/go-ipfs-ex/plugin/plugins/badgerds"
	pluginflatfs "github.com/IPFS-eX/go-ipfs-ex/plugin/plugins/flatfs"
	pluginipldgit "github.com/IPFS-eX/go-ipfs-ex/plugin/plugins/git"
	pluginlevelds "github.com/IPFS-eX/go-ipfs-ex/plugin/plugins/levelds"
)

// DO NOT EDIT THIS FILE
// This file is being generated as part of plugin build process
// To change it, modify the plugin/loader/preload.sh

func init() {
	Preload(pluginipldgit.Plugins...)
	Preload(pluginbadgerds.Plugins...)
	Preload(pluginflatfs.Plugins...)
	Preload(pluginlevelds.Plugins...)
}
