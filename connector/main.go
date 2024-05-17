package main

import (
	"context"
	"errors"

	"github.com/docker/docker/client"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/shellhub-io/shellhub/pkg/agent/connector"
	"github.com/shellhub-io/shellhub/pkg/envs"
	"github.com/shellhub-io/shellhub/pkg/middleware"
	log "github.com/sirupsen/logrus"
)

const (
	NilStatus          string = "nil"
	ConnectedStatus    string = "connected"
	DisconnectedStatus string = "disconnected"
	StartedStatus      string = "started"
	FailedStatus       string = "failed"
	StopedStatus       string = "stoped"
)

type Connection struct {
	cli            *client.Client
	stop           chan struct{}
	status         string
	serverAddress  string
	privateKeysDir string
}

func NewConnection(serverAddress, privateKeysDir string) *Connection {
	conn := Connection{
		cli:            nil,
		stop:           make(chan struct{}, 1),
		status:         NilStatus,
		serverAddress:  serverAddress,
		privateKeysDir: privateKeysDir,
	}

	return &conn
}

var (
	ErrConnectionEstablished error = errors.New("connection to Docker Remote Engine already established")
	ErrConnectionFailed      error = errors.New("connection to Docker Remote Engine failed")
	ErrConnectionUndefined   error = errors.New("connection to Docker Remote Engine is undefined")
)

// Connect trys to connect to a Remote Docker Engine on the address.
func (c *Connection) Connect(address string) error {
	if c.cli != nil {
		return ErrConnectionEstablished
	}

	cli, err := client.NewClientWithOpts(client.WithHost(address), client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	c.cli = cli

	c.status = ConnectedStatus

	return nil
}

// Ping pings the Remote Docker Engine and check if there no problems with the connection.
func (c *Connection) Ping(ctx context.Context) error {
	if c.cli == nil {
		return ErrConnectionUndefined
	}

	// TODO: Verify the [types.Ping] returned.
	if _, err := c.cli.Ping(ctx); err != nil {
		c.Disconnect() //nolint:errcheck

		return err
	}

	return nil
}

func (c *Connection) Disconnect() error {
	if c.cli == nil {
		return ErrConnectionUndefined
	}

	c.status = DisconnectedStatus

	return c.cli.Close()
}

// Start gets the connection with Remote Docker Engine and turns its containers into ShellHub devices.
func (c *Connection) Start(tenant Tenant) error {
	ctx, cancel := context.WithCancel(context.Background())
	// NOTE: Defer on the cancel guarantee that the goroutine that listens for containers will be destroied.
	defer cancel()

	if err := c.Ping(ctx); err != nil {
		return err
	}

	connc, err := connector.NewDockerConnectorWithClient(c.cli, c.serverAddress, string(tenant), c.privateKeysDir)
	if err != nil {
		return err
	}

	cherr := make(chan error)

	go func() {
		c.status = StartedStatus

		// NOTE: When father function returns, the Connector listening is stoped due to the context cancelation defined.
		if err := connc.Listen(ctx); err != nil {
			cherr <- err
		}
	}()

	for {
		select {
		case err := <-cherr:
			c.status = FailedStatus

			return err
		case <-c.stop:
			// NOTE: Force the context cancelation through `defer` executation.
			return nil
		}
	}
}

func (c *Connection) Stop() error {
	// TODO: Check if we can send in the channel.
	c.stop <- struct{}{}

	c.status = StopedStatus

	return nil
}

type Connector struct {
	Config      *Config
	Connections map[Tenant]*Connection
}

type Tenant string

func NewTenant(tenant string) (Tenant, error) {
	// TODO: Add validation to the conversion.
	return Tenant(tenant), nil
}

func (c *Connector) NewConnection() *Connection {
	conn := Connection{
		cli:            nil,
		stop:           make(chan struct{}, 1),
		status:         NilStatus,
		serverAddress:  c.Config.ServerAddress,
		privateKeysDir: c.Config.PrivateKeysDir,
	}

	return &conn
}

var (
	ErrConnectorConnectionNotFound     = errors.New("connection not found")
	ErrConnectorConnectionAlreadyAdded = errors.New("connectio already added")
)

func (c *Connector) GetConnection(tenant Tenant) (*Connection, error) {
	conn, ok := c.Connections[tenant]
	if !ok {
		return nil, ErrConnectorConnectionNotFound
	}

	return conn, nil
}

func (c *Connector) AddConnection(tenant Tenant, conn *Connection) error {
	if _, err := c.GetConnection(tenant); err == nil {
		return ErrConnectorConnectionAlreadyAdded
	}

	c.Connections[tenant] = conn

	return nil
}

func (c *Connector) DelConnection(tenant Tenant) error {
	if _, err := c.GetConnection(tenant); err != nil {
		return err
	}

	delete(c.Connections, tenant)

	return nil
}

type Config struct {
	PrivateKeysDir string `env:"PRIVATE_KEYS_DIR,default=/tmp/shellhub"`
	ServerAddress  string `env:"SERVER_ADDRESS,default=http://localhost/"`
}

func main() {
	log.SetFormatter(new(log.JSONFormatter))
	log.Info("starting ShellHub Connector")

	config, err := envs.ParseWithPrefix[Config]("API_")
	if err != nil {
		log.Fatal("failed to parse the environmental variables")
	}

	log.WithFields(log.Fields{
		"private_keys_dir": config.PrivateKeysDir,
		"server_address":   config.ServerAddress,
	}).Info("environemental variables loaded")

	connector := &Connector{
		Config:      config,
		Connections: make(map[Tenant]*Connection, 0),
	}

	const ServerHTTPAddress = ":8080"

	server := echo.New()
	server.Use(echoMiddleware.RequestID())
	server.Use(middleware.Log)

	server.HideBanner = true

	/*
	   Enable Connector connection for this namespace.

	   ```
	   PATCH /api/namespaces/:tenant
	   { "connector": "localhost:2375" }
	   ```

	   Operate on the Connector connection.

	   The actions avaliables are:
	   - connect
	   Trys to connect to the Docker Remote Engine registered.
	   - start
	   Turn the conbtainers on the Docker Remote Engine registered into ShellHub Devices.
	   - status
	   Return the Docker Remote Engine Status.
	   - stop
	   Stop the ShellHub Agents registered.
	   - disconnect
	   Disconnect from the Docker Remote Engine.

	   ```
	   POST /api/namespaces/:tenant/connector
	   { "action": "connect" }
	   ```
	*/

	endpoints := server.Group("")
	endpoints.POST("/connect/:tenant", func(c echo.Context) error {
		// NOTE: Try to apply the "parse don't validate" pattern.
		tenant, _ := NewTenant(c.Param("tenant"))

		if _, err := connector.GetConnection(tenant); err == nil {
			return ErrConnectionEstablished
		}

		conn := connector.NewConnection()

		if err := conn.Connect("tcp://localhost:2375"); err != nil { // 2375
			return ErrConnectionFailed
		}

		connector.Connections[tenant] = conn

		return nil
	})

	endpoints.POST("/disconnect/:tenant", func(c echo.Context) error {
		tenant, _ := NewTenant(c.Param("tenant"))

		conn, err := connector.GetConnection(tenant)
		if err != nil {
			return err
		}

		if err := conn.Stop(); err != nil {
			return err
		}

		if err := conn.Disconnect(); err != nil { // 2375
			return err
		}

		return connector.DelConnection(tenant)
	})

	endpoints.POST("/start/:tenant", func(c echo.Context) error {
		tenant, _ := NewTenant(c.Param("tenant"))

		conn, err := connector.GetConnection(tenant)
		if err != nil {
			return err
		}

		if conn.status == StartedStatus {
			return errors.New("connection already started")
		}

		go conn.Start(tenant) //nolint:errcheck

		return nil
	})

	endpoints.POST("/stop/:tenant", func(c echo.Context) error {
		tenant, _ := NewTenant(c.Param("tenant"))

		conn, err := connector.GetConnection(tenant)
		if err != nil {
			return err
		}

		if conn.status != StartedStatus {
			return errors.New("connection isn't started")
		}

		return conn.Stop()
	})

	endpoints.POST("/ping/:tenant", func(c echo.Context) error {
		tenant, _ := NewTenant(c.Param("tenant"))

		conn, _ := connector.GetConnection(tenant)
		if err != nil {
			return err
		}

		return conn.Ping(c.Request().Context())
	})

	log.Infof("http server listing on %s", ServerHTTPAddress)

	log.Fatal(server.Start(ServerHTTPAddress)) //nolint:errcheck
}
