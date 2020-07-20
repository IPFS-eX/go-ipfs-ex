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

func (s *ScanAPI) PublishFile(ctx context.Context, hash string, h host.Host) error {
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
	fmt.Println("push addrs to scan", addrs)
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
	for _, a := range addrs {
		if ScanCfg.IgnoreIPv6 && strings.Contains(a.String(), "ip6") {
			continue
		}
		if ScanCfg.IgnoreLocalHost && strings.Contains(a.String(), "192") {
			continue
		}
		_, err := tkServer.AnnounceRequestCompleteTorrent(&pm.CompleteTorrentReq{
			InfoHash: []byte(hash),
			NodeAddr: []byte(a.String()),
		}, id)
		cLog.Debugf("push hash: %v, host %v, err %v\n", hash, h, err)
	}
	return nil
}

func (s *ScanAPI) GetFilePeers(ctx context.Context, hash string) ([]string, error) {
	id, err := tkActClient.P2pConnect(ScanCfg.Bootstrap[0])
	cLog.Debugf("connect id %v, err %v", id, err)
	if err != nil {
		return nil, err
	}
	if len(id) == 0 {
		return nil, fmt.Errorf("id is null")
	}
	if tkServer == nil {
		return nil, fmt.Errorf("tracker server is nil")
	}
	peerRest, err := tkServer.AnnounceRequestTorrentPeers(&pm.GetTorrentPeersReq{
		InfoHash: []byte(hash),
		NumWant:  uint64(ScanCfg.MaxNumWant),
	}, id)

	if err != nil {
		return nil, err
	}
	if peerRest == nil {
		return nil, fmt.Errorf("peer response is nil")
	}
	return peerRest.Peers, nil
}
