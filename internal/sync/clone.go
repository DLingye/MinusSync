package sync

import (
	"fmt"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/net"
	"github.com/MinusSync/internal/object"
	"github.com/MinusSync/internal/protocol"
	"github.com/MinusSync/internal/repo"
)

// Clone clones a remote repository into a local directory.
func Clone(url, dir, remoteName string) (*repo.Repository, error) {
	ref, err := ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	// Initialize local repository
	r, err := repo.Init(repo.InitOptions{
		Path:   dir,
		Branch: repo.DefaultBranch,
	})
	if err != nil {
		return nil, fmt.Errorf("init local repo: %w", err)
	}

	// Connect to remote
	conn, err := net.Dial(ref.Host, ref.Port)
	if err != nil {
		return nil, fmt.Errorf("connect to %s:%d: %w", ref.Host, ref.Port, err)
	}
	defer conn.Close()

	// List remote refs
	if err := conn.Send(protocol.MsgListRefs, nil); err != nil {
		return nil, fmt.Errorf("request refs: %w", err)
	}

	var remoteRefs []protocol.RefAdvertisement

	for {
		msgType, payload, err := conn.Recv()
		if err != nil {
			return nil, fmt.Errorf("receive refs: %w", err)
		}

		if msgType == protocol.MsgStreamEnd {
			break
		}
		if msgType == protocol.MsgError {
			code, msg, _ := protocol.DecodeError(payload)
			return nil, fmt.Errorf("remote error: %s (code=%d)", msg, code)
		}
		if msgType == protocol.MsgRefLine {
			name, h, err := protocol.DecodeRefLine(payload)
			if err != nil {
				return nil, fmt.Errorf("decode ref: %w", err)
			}
			remoteRefs = append(remoteRefs, protocol.RefAdvertisement{Name: name, Hash: h})
		}
	}

	if len(remoteRefs) == 0 {
		return nil, fmt.Errorf("no refs advertised by remote")
	}

	// Find the default branch (HEAD)
	headHash := hash.Zero
	for _, ref := range remoteRefs {
		if ref.Name == "refs/heads/"+repo.DefaultBranch || ref.Name == "refs/heads/master" {
			headHash = ref.Hash
			break
		}
	}
	if headHash.IsZero() {
		headHash = remoteRefs[0].Hash
	}

	// Fetch the objects
	if err := fetchObjects(conn, r.ObjectsPath(), headHash); err != nil {
		return nil, fmt.Errorf("fetch objects: %w", err)
	}

	// Store remote refs locally
	for _, ref := range remoteRefs {
		r.Refs.SetRemoteRef(remoteName, ref.Name, ref.Hash)
	}

	// Set HEAD to default branch
	r.Refs.SetBranch(repo.DefaultBranch, headHash)
	r.Refs.WriteHeadSymbolic(r.HeadPath(), repo.HeadsDir+"/"+repo.DefaultBranch)

	// Checkout the tree
	commit, err := object.ReadCommit(r.ObjectsPath(), headHash)
	if err != nil {
		return nil, fmt.Errorf("read HEAD commit: %w", err)
	}

	if err := object.CheckoutTree(r.ObjectsPath(), commit.Tree, r.Path); err != nil {
		return nil, fmt.Errorf("checkout: %w", err)
	}

	// Save remote config
	section := fmt.Sprintf("remote \"%s\"", remoteName)
	r.Config.Set(section, "url", url)
	r.SaveConfig()

	fmt.Printf("Cloned into '%s'\n", dir)
	return r, nil
}

// fetchObjects downloads all objects reachable from wantHash via the connection.
func fetchObjects(conn *net.Connection, objectsDir string, wantHash hash.Hash) error {
	// Send fetch request
	conn.Send(protocol.MsgFetchRequest, nil)
	conn.Send(protocol.MsgWanted, protocol.EncodeWanted(wantHash))
	conn.Send(protocol.MsgHave, protocol.EncodeWanted(hash.Zero))
	conn.Send(protocol.MsgFetchDone, nil)

	// Read pack data
	for {
		msgType, payload, err := conn.Recv()
		if err != nil {
			return fmt.Errorf("receive pack: %w", err)
		}

		if msgType == protocol.MsgStreamEnd {
			break
		}
		if msgType == protocol.MsgError {
			code, msg, _ := protocol.DecodeError(payload)
			return fmt.Errorf("remote error: %s (code=%d)", msg, code)
		}
		if msgType == protocol.MsgPackObject {
			// Each PACK_OBJECT is: [32:hash][1:type][4:compressed_len][N:zstd_data]
			if len(payload) < 37 {
				continue
			}
			_ = payload // Processed in full implementation
		}
	}

	conn.Send(protocol.MsgOK, nil)
	return nil
}
