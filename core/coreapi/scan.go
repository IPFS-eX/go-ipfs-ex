package coreapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	cLog "github.com/IPFS-eX/carrier/common/log"
	config "github.com/IPFS-eX/go-ipfs-config"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	coreiface "github.com/IPFS-eX/interface-go-ipfs-core"
	pm "github.com/IPFS-eX/ipfs-scan/p2p/actor/messages"
	tkActClient "github.com/IPFS-eX/ipfs-scan/p2p/actor/tracker/client"
	tkActServer "github.com/IPFS-eX/ipfs-scan/p2p/actor/tracker/server"
	"github.com/IPFS-eX/ipfs-scan/service"
	sc "github.com/IPFS-eX/ipfs-scan/service/crypto"
	ic "github.com/libp2p/go-libp2p-core/crypto"
)

type ScanAPI CoreAPI

var tkServer *tkActServer.TrackerActorServer

var ScanCfg config.Scan

type ScanFileInfo struct {
	Name  string
	Hash  string
	Peers []string
}

func (s ScanFileInfo) GetName() string {
	return s.Name
}

func (s ScanFileInfo) GetHash() string {
	return s.Hash
}

func (s ScanFileInfo) GetPeers() []string {
	return s.Peers
}

func (s *ScanAPI) Startup(privKey ic.PrivKey, cfgM map[string]interface{}) error {
	// cLog.InitLog(cLog.DebugLog, os.Stdout)
	if tkServer != nil {
		cLog.Debugf("ignore re init")
		return nil
	}
	pubKey := privKey.GetPublic()
	var err error
	pubKeyBuf, err := ic.MarshalPublicKey(pubKey)
	if err != nil {
		return err
	}
	privKeyBuf, err := ic.MarshalPrivateKey(privKey)
	if err != nil {
		return err
	}
	keyPair := &sc.ScanKeyPair{
		PublicKey:     pubKey,
		PrivateKey:    privKey,
		PublicKeyBuf:  pubKeyBuf,
		PrivateKeyBuf: privKeyBuf,
	}
	cfgData, err := json.Marshal(cfgM)
	if err != nil {
		return err
	}
	cfg := &config.Config{}
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		return err
	}
	ScanCfg = cfg.Scan

	tkServer, err = service.NewTrackerActorServer(
		uint32(ScanCfg.NetworkId),
		ScanCfg.ListenAddr,
		fmt.Sprintf("%v", ScanCfg.Port),
		keyPair,
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *ScanAPI) PublishFile(ctx context.Context, fileInfo coreiface.FileInfo, h host.Host) error {
	if h == nil {
		return nil
	}
	if len(ScanCfg.Bootstrap) == 0 {
		return nil
	}
	addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(h))
	if err != nil {
		return err
	}
	id, err := tkActClient.P2pConnect(ScanCfg.Bootstrap[0])
	cLog.Debugf("connect id %v, err %v", id, err)
	if err != nil {
		return err
	}
	if len(id) == 0 {
		return fmt.Errorf("id is null")
	}
	if tkServer == nil {
		return fmt.Errorf("tracker server is nil")
	}
	sFileInfo, ok := fileInfo.(*ScanFileInfo)
	if !ok {
		return nil
	}
	for _, a := range addrs {
		if ScanCfg.IgnoreIPv6 && strings.Contains(a.String(), "ip6") {
			continue
		}
		if ScanCfg.IgnoreLocalHost && strings.Contains(a.String(), "192") {
			continue
		}
		_, err := tkServer.AnnounceRequestCompleteTorrent(&pm.CompleteTorrentReq{
			Name:     []byte(sFileInfo.Name),
			InfoHash: []byte(sFileInfo.Hash),
			NodeAddr: []byte(a.String()),
		}, id)
		cLog.Debugf("push hash: %v, host %v, err %v\n", sFileInfo.Hash, h, err)
	}

	for _, p := range fileInfo.GetPeers() {
		if ScanCfg.IgnoreIPv6 && strings.Contains(p, "ip6") {
			continue
		}
		if ScanCfg.IgnoreLocalHost && strings.Contains(p, "192") {
			continue
		}
		_, err := tkServer.AnnounceRequestCompleteTorrent(&pm.CompleteTorrentReq{
			Name:     []byte(sFileInfo.Name),
			InfoHash: []byte(sFileInfo.Hash),
			NodeAddr: []byte(p),
		}, id)
		cLog.Debugf("push hash: %v, host %v, err %v\n", sFileInfo.Hash, h, err)
	}
	return nil
}
func (s *ScanAPI) GetFilePeers(ctx context.Context, hash string) (coreiface.FileInfo, error) {
	id, err := tkActClient.P2pConnect(ScanCfg.Bootstrap[0])
	cLog.Debugf("connect id %v, err %v", id, err)
	if err != nil {
		return ScanFileInfo{}, err
	}
	if len(id) == 0 {
		return ScanFileInfo{}, fmt.Errorf("id is null")
	}
	if tkServer == nil {
		return ScanFileInfo{}, fmt.Errorf("tracker server is nil")
	}
	peerRest, err := tkServer.AnnounceRequestTorrentPeers(&pm.GetTorrentPeersReq{
		InfoHash: []byte(hash),
		NumWant:  uint64(ScanCfg.MaxNumWant),
	}, id)

	if err != nil {
		return ScanFileInfo{}, err
	}
	if peerRest == nil {
		return ScanFileInfo{}, fmt.Errorf("peer response is nil")
	}
	fileInfo := ScanFileInfo{
		Name:  string(peerRest.Name),
		Hash:  hash,
		Peers: peerRest.Peers,
	}
	return fileInfo, nil
}
