package main

/*
 * listeners.go
 * Handle general listeners
 * By J. Stuart McMurray
 * Created 20220326
 * Last Modified 20220402
 */

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/magisterquis/jec2/pkg/common"
	"golang.org/x/crypto/ssh"
)

const (
	/* defaultSSHBanner is the default SSH version string sent to clients. */
	defaultSSHBanner = "SSH-2.0-OpenSSH_8.8"
)

var (
	/* sessionCounter counts connected sessions and is used as a key. */
	sessionCounter uint64

	/* sshConf is the current SSH config. */
	sshConf  *ssh.ServerConfig
	sshConfL sync.RWMutex
)

// GenSSHConfig (re)generates the SSH server config.  If the banner is not the
// empty string it will be used in place of the default SSH banner.
func GenSSHConfig(banner string) error {
	/* Work out the banner to send. */
	if "" == banner {
		banner = defaultSSHBanner
	}

	/* Server config itself. */
	conf := &ssh.ServerConfig{
		PublicKeyCallback: sshPublicKeyCallback,
		ServerVersion:     banner,
	}

	/* Get the SSH key. */
	k, _, made, err := common.GetOrMakeKey(common.ServerKeyFile)
	if nil != err {
		return fmt.Errorf("get/make key: %w", err)
	}
	fp := ssh.FingerprintSHA256(k.PublicKey())
	if made {
		log.Printf("Made server key in %s", common.ServerKeyFile)
	}
	log.Printf("Server key fingerprint: %s", fp)
	SetServerFP(fp)
	conf.AddHostKey(k)

	/* Make the public side as well. */
	pkf := common.ServerKeyFile + ".pub"
	if err := os.WriteFile(
		pkf,
		ssh.MarshalAuthorizedKey(k.PublicKey()),
		0644,
	); nil != err {
		log.Printf(
			"Error writing server public key to %s: %s",
			pkf,
			err,
		)
	}

	/* Update the current config. */
	sshConfL.Lock()
	defer sshConfL.Unlock()
	sshConf = conf

	return nil
}

// HandleSSH handles a new SSH client.
func HandleSSH(c net.Conn) {
	tag := "SSH:" + c.RemoteAddr().String()

	/* Get SSH config.  If we don't have one, something's gone wrong. */
	defer c.Close()
	sshConfL.RLock()
	conf := sshConf
	sshConfL.RUnlock()
	if nil == conf {
		log.Printf("[%s] SSH config missing?", tag)
		return
	}

	/* Upgrade to SSH */
	sc, chans, reqs, err := ssh.NewServerConn(c, conf)
	if nil != err {
		log.Printf("[%s] Handshake error: %s", tag, err)
		return
	}
	var (
		ct string /* Connection type */
		hf func(  /* Handler function */
			string,
			*ssh.ServerConn,
			<-chan ssh.NewChannel,
			<-chan *ssh.Request,
		) error
	)

	/* Handle the connection. */
	switch t := sc.Permissions.Extensions["key-type"]; t {
	case KeyTypeOperator:
		tag = fmt.Sprintf("%s@%s", sc.User(), sc.RemoteAddr())
		log.Printf(
			"[%s] Operator connected with key %s",
			tag,
			sc.Permissions.Extensions["fingerprint"],
		)
		ct = "Operator"
		hf = HandleOperator
	case KeyTypeImplant:
		tag = fmt.Sprintf("%s", sc.Permissions.Extensions["snum"])
		log.Printf(
			"[%s] Implant connected with key %s and username %q",
			tag,
			sc.Permissions.Extensions["fingerprint"],
			sc.User(),
		)
		ct = "Implant"
		hf = HandleImplant
	default:
		log.Printf("[%s] Unknown key type %s", tag, t)
		return
	}

	/* Service the connection. */
	go func() {
		if err := hf(tag, sc, chans, reqs); nil != err {
			log.Printf("[%s] %s service error: %s", tag, ct, err)
		}
	}()

	/* Nice log for when client disconnects. */
	err = sc.Wait()
	//if nil == err || errors.Is(err, io.EOF) {
	//	log.Printf("[%s] %s disconnected", tag, ct)
	//	return
	//}
	//log.Printf("[%s] %s disconnected: %s", tag, ct, err)

}

/* sshPublkcKeyCallback is used as the PublicKeyCallback in the SSH server
config. */
func sshPublicKeyCallback(
	conn ssh.ConnMetadata,
	key ssh.PublicKey,
) (*ssh.Permissions, error) {
	var snum string

	/* See if we know this key. */
	t := GetAllowedKeyType(key)
	switch t {
	case KeyTypeOperator:
	case KeyTypeImplant:
		n := atomic.AddUint64(&sessionCounter, 1)
		snum = "m" + strconv.FormatUint(n, 10)
	case KeyTypeUnknown:
		return nil, fmt.Errorf("unknown key")
	default: /* Unpossible */
		return nil, fmt.Errorf("unknown allowed key type %s", t)
	}

	/* We must know the key, let the handler know. */
	return &ssh.Permissions{
		Extensions: map[string]string{
			"key-type":    t,
			"fingerprint": ssh.FingerprintSHA256(key),
			"snum":        snum,
		},
	}, nil
}
