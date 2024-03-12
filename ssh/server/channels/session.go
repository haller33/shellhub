package channels

import (
	"sync"

	gliderssh "github.com/gliderlabs/ssh"
	"github.com/shellhub-io/shellhub/ssh/session"
	log "github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"
)

const (
	// Once the session has been set up, a program is started at the remote end.  The program can be a shell, an
	// application program, or a subsystem with a host-independent name.  Only one of these requests can succeed per
	// channel
	//
	// https://www.rfc-editor.org/rfc/rfc4254#section-6.5
	ShellRequestType = "shell"
	// This message will request that the server start the execution of the given command.  The 'command' string may
	// contain a path.  Normal precautions MUST be taken to prevent the execution of unauthorized commands.
	//
	// https://www.rfc-editor.org/rfc/rfc4254#section-6.5
	ExecRequestType = "exec"
	// This last form executes a predefined subsystem.  It is expected that these will include a general file transfer
	// mechanism, and possibly other features.  Implementations may also allow configuring more such mechanisms.  As
	// the user's shell is usually used to execute the subsystem, it is advisable for the subsystem protocol to have a
	// "magic cookie" at the beginning of the protocol transaction to distinguish it from arbitrary output generated
	// by shell initialization scripts, etc.  This spurious output from the shell may be filtered out either at the
	// server or at the client.
	//
	// https://www.rfc-editor.org/rfc/rfc4254#section-6.5
	SubsystemRequestType = "subsystem"
	//  A pseudo-terminal can be allocated for the session by sending the following message.
	//
	// The 'encoded terminal modes' are described in Section 8.  Zero dimension parameters MUST be ignored.  The
	// character/row dimensions override the pixel dimensions (when nonzero).  Pixel dimensions refer to the drawable
	// area of the window.
	//
	// https://www.rfc-editor.org/rfc/rfc4254#section-6.2
	PtyRequestType = "pty-req"
	// When the window (terminal) size changes on the client side, it MAY send a message to the other side to inform it
	// of the new dimensions.
	//
	// https://www.rfc-editor.org/rfc/rfc4254#section-6.7
	WindowChangeRequestType = "window-change"
	KeepAliveRequestType    = "keepalive"
)

// DefaultSessionHandler is the default handler for session's channel.
//
// A session is a remote execution of a program.  The program may be a shell, an application, a system command, or some
// built-in subsystem. It may or may not have a tty, and may or may not involve X11 forwarding.
//
// https://www.rfc-editor.org/rfc/rfc4254#section-6
func DefaultSessionHandler(_ *gliderssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx gliderssh.Context) {
	reject := func(err error, msg string) {
		log.WithError(err).Error(msg)

		newChan.Reject(gossh.ConnectionFailed, msg) //nolint:errcheck
	}

	defer conn.Close()

	sess, ok := ctx.Value("session").(*session.Session)
	if !ok {
		reject(nil, "failed to recover the session created")

		return
	}

	defer sess.Finish() //nolint:errcheck

	log.WithFields(
		log.Fields{
			"uid":      sess.UID,
			"device":   sess.Device.UID,
			"username": sess.Username,
			"ip":       sess.IPAddress,
		}).Info("session channel started")
	defer log.WithFields(
		log.Fields{
			"uid":      sess.UID,
			"device":   sess.Device.UID,
			"username": sess.Username,
			"ip":       sess.IPAddress,
		}).Info("session channel done")

	client, clientReqs, err := newChan.Accept()
	if err != nil {
		reject(err, "failed to accept the channel opening")

		return
	}

	defer client.Close()

	agent, agentReqs, err := sess.Agent.OpenChannel(SessionChannel, nil)
	if err != nil {
		reject(err, "failed to open the 'session' channel on agent")

		return
	}

	defer agent.Close()

	mu := new(sync.Mutex)

	started := false

	for {
		select {
		case <-ctx.Done():
			log.Info("context has done")

			return
		case req, ok := <-sess.AgentGlobalReqs:
			if !ok {
				log.Trace("global requests is closed")

				return
			}

			log.Debugf("global request from agent: %s", req.Type)

			switch req.Type {
			case KeepAliveRequestType:
				if err := sess.KeepAlive(); err != nil {
					log.Error(err)

					return
				}

				if err := req.Reply(false, nil); err != nil {
					log.Error(err)
				}
			default:
				if req.WantReply {
					if err := req.Reply(false, nil); err != nil {
						log.Error(err)
					}
				}
			}
		case req, ok := <-clientReqs:
			if !ok {
				log.Trace("client requests is closed")

				return
			}

			log.Debugf("request from client to agent: %s", req.Type)

			ok, err := agent.SendRequest(req.Type, req.WantReply, req.Payload)
			if err != nil {
				continue
			}

			switch req.Type {
			// Once the session has been set up, a program is started at the remote end.  The program can be a shell, an
			// application program, or a subsystem with a host-independent name.  **Only one of these requests can
			// succeed per channel.**
			//
			// https://www.rfc-editor.org/rfc/rfc4254#section-6.5
			case ShellRequestType, ExecRequestType, SubsystemRequestType:
				if !started {
					// It is RECOMMENDED that the reply to these messages be requested and checked.  The client SHOULD
					// ignore these messages.
					//
					// https://www.rfc-editor.org/rfc/rfc4254#section-6.5
					if req.WantReply {
						if err := req.Reply(ok, []byte{}); err != nil {
							log.WithError(err).Error("failed to reply the client with right response for pipe request type")

							return
						}

						mu.Lock()
						started = true
						mu.Unlock()

						log.WithFields(
							log.Fields{
								"uid":      sess.UID,
								"device":   sess.Device.UID,
								"username": sess.Username,
								"ip":       sess.IPAddress,
								"type":     req,
							}).Info("session type set")

						if req.Type == ShellRequestType {
							if err := sess.Announce(client); err != nil {
								log.WithError(err).WithFields(log.Fields{
									"uid":      sess.UID,
									"device":   sess.Device.UID,
									"username": sess.Username,
									"ip":       sess.IPAddress,
									"type":     req,
								}).Warn("failed to get the namespace announcement")
							}
						}

						// The server SHOULD NOT halt the execution of the protocol stack when starting a shell or a
						// program.  All input and output from these SHOULD be redirected to the channel or to the
						// encrypted tunnel.
						//
						// https://www.rfc-editor.org/rfc/rfc4254#section-6.5

						go pipe(sess, client, agent, req.Type)
					}
				} else {
					log.WithError(err).WithFields(log.Fields{
						"uid":      sess.UID,
						"device":   sess.Device.UID,
						"username": sess.Username,
						"ip":       sess.IPAddress,
						"type":     req,
					}).Warn("tried to start and forbidden request type")

					if err := req.Reply(false, nil); err != nil {
						log.WithError(err).Error("failed to reply the client when data pipe already started")

						return
					}
				}
			case PtyRequestType:
				var pty session.Pty

				if err := gossh.Unmarshal(req.Payload, &pty); err != nil {
					reject(nil, "failed to recover the session dimensions")
				}

				sess.Pty = pty

				if req.WantReply {
					// req.Reply(ok, nil) //nolint:errcheck
					if err := req.Reply(ok, nil); err != nil {
						log.WithError(err).Error("failed to reply for pty-req")

						return
					}
				}
			case WindowChangeRequestType:
				var dimensions session.Dimensions

				if err := gossh.Unmarshal(req.Payload, &dimensions); err != nil {
					reject(nil, "failed to recover the session dimensions")
				}

				sess.Pty.Columns = dimensions.Columns
				sess.Pty.Rows = dimensions.Rows

				if req.WantReply {
					req.Reply(ok, nil) //nolint:errcheck
				}
			default:
				if req.WantReply {
					if err := req.Reply(ok, nil); err != nil {
						log.WithError(err).Error("failed to reply for window-change")

						return
					}
				}
			}
		case req, ok := <-agentReqs:
			if !ok {
				log.Trace("agent requests is closed")

				return
			}

			log.Debugf("request from agent to client: %s", req.Type)

			ok, err := client.SendRequest(req.Type, req.WantReply, req.Payload)
			if err != nil {
				continue
			}

			if req.WantReply {
				if err := req.Reply(ok, nil); err != nil {
					log.WithError(err).Error("failed to reply the agent request")

					return
				}
			}
		}
	}
}