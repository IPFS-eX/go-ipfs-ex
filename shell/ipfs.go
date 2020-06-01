/**
 * Description:
 * Author: Yihen.Liu
 * Create: 2020-04-27
 */
package shell

import (
	"os"

	"github.com/ETHFSx/go-ipfs/shell/ipfs"
)

const DefaultIpfsDir = "~/.ipfs"

func StartDaemon() {
	ipfs.MainStart("daemon")
}

func InitWorkspace() {
	path := os.Getenv("IPFS_PATH")
	if path == "" {
		path = DefaultIpfsDir
	}
	if _, err := os.Stat("path"); err != nil {
		ipfs.MainStart("init")
	}
}
