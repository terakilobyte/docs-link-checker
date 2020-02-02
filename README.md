# docs-link-checker

Usage is covered in detail in the help text once you run the tool. It can read entire directories, files, or from stdin.
It can output to stdout or to a file.

URLs must begin with `https://` or `http://` for them to match. Once matched, they are put into a queue. The queue is processed concurrently and the URLs are checked.

Any URL that returns a non 200 status will be noted. Additionally, if the URL is a Github link, the repo will be interrogated for the date of last activity. If the last activity is over a year ago, the repo will be noted.

Due to Github API rate limiting, an API key is required. This
can be defined either in $HOME/.docs-link-checker.yaml or in your environment.

## Getting Started

This project requires the use of [Go modules](https://github.com/golang/go/wiki/Modules).

### Requirements

* Go 1.13
* Github API Key

### Procedure

Clone this repository to a location outside of your GOPATH:

```sh
echo $GOPATH
```

> Don't clone into the printed path! ^^^^^

```sh
git clone git@github.com:terakilobyte/docs-link-checker
```

### Environmental Variables

You must provide an environmental variable called `GIT_REPO_TOKEN`.
This is to avoid rate limiting from Github.

For **zshrc** users:

```sh
echo "GIT_REPO_TOKEN='XXXXXXX...'" >> ~/.zshrc
```

For **bash** users:

```sh
echo "GIT_REPO_TOKEN='XXXXXXX...'" >> ~/.bashrc
```

### Go modules

This project uses [Go modules](https://github.com/golang/go/wiki/Modules) for dependency management.
When using `go run`, use the `-mod=readonly` flag to avoid unintentionally updating dependency checksums.
If you see the error `go: updates to go.mod needed, disabled by -mod=readonly`,
run `go mod tidy` to update the `go.mod` and `go.sum` files.
