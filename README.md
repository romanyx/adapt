[![Build Status](https://travis-ci.org/romanyx/adapt.svg?branch=master)](https://travis-ci.org/romanyx/adapt)

`adapt` generates adapter type to allow the use of ordinary functions as the interface.

```bash
go get -u github.com/romanyx/adapt
```

You can pass package and interface names to generate adapter:

```bash
$ adapt io Reader
type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) {
	return f(p)
}
```

You also can call `adapt` inside a package folder to generate adapter for some of its interfaces:

```bash
$ cd $GOPATH/src/github.com/romanyx/polluter Polluter
$ adapt Polluter
type polluterFunc func(io.Reader) error

func (f polluterFunc) Pollute(r io.Reader) error {
	return f(r)
}
```

It comes in handy for Unit testing with TDD.

You can use `adapt` from Vim with [vim-go-adapt](https://github.com/romanyx/vim-go-adapt)
