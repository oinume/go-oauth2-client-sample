[![CI](https://github.com/oinume/go-oauth2-client-sample/workflows/ci/badge.svg?query=branch%3Amaster)](https://github.com/oinume/go-oauth2-client-sample/actions/workflows/ci.yml?query=branch%3Amaster)

# go-oauth2-client-sample
Simple OAuth2 client implementation in Go. See following articles for details.

- https://journal.lampetty.net/entry/oauth2-client-handson-in-go-setup
- https://journal.lampetty.net/entry/oauth2-client-handson-in-go-authorization-code-grant

## Requirements

- Go 1.10 or later
- make


## How to build binary

```shell
make build
```

## How to run

```shell
cp .env.sample .env
```

Edit .env with your CLIENT_ID and CLIENT_SECRET from [Google APIs](https://console.developers.google.com/apis/credentials).

And then run the server.

```shell
source .env
make run
```

You can access to http://localhost:2345 with a web browser after executing the command.

Change `PORT` on .env if you want to change listening port.