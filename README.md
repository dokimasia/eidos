# eidos

A Go project using [ergon](https://go.thesmos.sh/ergon) for the
build, test, lint, and release lifecycle.

## Development

```
make bootstrap    # install dev tools
make install      # go mod download
make check        # full pre-merge gate (mod verify + lint + test + checks)
make build        # compile every module's source
```

`make help` lists every target.

## License

See [LICENSE](LICENSE).
