# Artifacts mover

This small utility uploads artifacts produced by Project Reality server. It supports multiple servers.

A server with PRDemo and JSON summaries enabled is required.

It support SCP, SFTP and HTTPS upload protocols. 

## Algorithm

- Watches each server's configured directories for new PRDemo, BF2Demo and summary files.
- Matches files created around the same time into a single round.
- Once a round is complete (or a timeout is reached for a round still in progress), its files are uploaded.
- After a successful upload, a Discord notification with links to the round's files is sent.

## Configuration

Each server is configured with its artifact directories, an upload method (SCP or HTTPS) and, optionally, Discord notifications. See [config.sample.yaml](./config.sample.yaml) for a full example.

## Usage

Requires a `DISCORD_TOKEN` environment variable for the notification bot.

```sh
go build -o bin/artifacts-mover ./cmd
./bin/artifacts-mover -config config.yaml
```

A Docker image is also provided; see the [Dockerfile](./Dockerfile).

