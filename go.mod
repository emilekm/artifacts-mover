module github.com/emilekm/artifacts-mover

go 1.26.2

require (
	github.com/bwmarrin/discordgo v0.29.0
	github.com/emilekm/go-prbf2 v0.0.0-20251211144904-8368b37fb63f
	github.com/fogleman/gg v1.3.0
	github.com/fsnotify/fsnotify v1.8.0
	github.com/goccy/go-yaml v1.15.23
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	go.etcd.io/bbolt v1.4.3
	golang.org/x/sync v0.20.0
)

require (
	github.com/ghostiam/binstruct v1.3.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	go.uber.org/mock v0.6.0 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/image v0.32.0 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/tools v0.44.0 // indirect
)

tool (
	go.uber.org/mock/mockgen
	golang.org/x/tools/cmd/stringer
)
