# Dragonfly2压测

This is an example file with default selections.

## Install

```
git clone git@github.com:Louhwz/dragonfly2-stress_test.git
GOOS=linux go build -o stress
```

## Usage

```
./stress docker.io/library/busybox:latest  -r=20
```
You can also use `./stress -h` to see more detailed help message.

## Contributing

PRs accepted.

## License

MIT © Richard McRichface
