package sync

import (
	"fmt"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/net"
	"github.com/MinusSync/internal/protocol"
	"github.com/MinusSync/internal/repo"
)

// Fetch downloads objects and refs from a remote without modifying the working tree.
func Fetch(r *repo.Repository, remoteName string) error {
	ref, err := GetRemote(r.Config, remoteName)
	if err != nil {
		return fmt.Errorf("get remote: %w", err)
	}

	conn, err := net.Dial(ref.Host, ref.Port)
	if err != nil {
		return fmt.Errorf("connect to %s:%d: %w", ref.Host, ref.Port, err)
	}
	defer conn.Close()

	// List remote refs
	if err := conn.Send(protocol.MsgListRefs, nil); err != nil {
		return err
	}

	var remoteRefs []protocol.RefAdvertisement
	for {
		msgType, payload, err := conn.Recv()
		if err != nil {
			return err
		}
		if msgType == protocol.MsgStreamEnd {
			break
		}
		if msgType == protocol.MsgRefLine {
			name, h, err := protocol.DecodeRefLine(payload)
			if err != nil {
				return err
			}
			remoteRefs = append(remoteRefs, protocol.RefAdvertisement{Name: name, Hash: h})
		}
	}

	// Fetch HEAD
	for _, remoteRef := range remoteRefs {
		if remoteRef.Name != "HEAD" {
			continue
		}
		localHash, err := r.Refs.GetRemoteRef(remoteName, "HEAD")
		if err == nil && localHash.Equal(remoteRef.Hash) {
			continue // Already up to date
		}

		conn.Send(protocol.MsgFetchRequest, nil)
		conn.Send(protocol.MsgWanted, protocol.EncodeWanted(remoteRef.Hash))
		if !localHash.IsZero() {
			conn.Send(protocol.MsgHave, protocol.EncodeWanted(localHash))
		} else {
			conn.Send(protocol.MsgHave, protocol.EncodeWanted(hash.Zero))
		}
		conn.Send(protocol.MsgFetchDone, nil)

		for {
			msgType, payload, err := conn.Recv()
			if err != nil {
				return err
			}
			if msgType == protocol.MsgStreamEnd {
				break
			}
			if msgType == protocol.MsgPackObject {
				_ = payload
			}
		}

		r.Refs.SetRemoteRef(remoteName, "HEAD", remoteRef.Hash)
	}

	fmt.Printf("Fetched from %s\n", remoteName)
	return nil
}
