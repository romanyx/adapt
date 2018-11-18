package main

import (
	"io"
	"net/http"
)

type local int64

type variadic interface {
	Func(...io.Reader)
}

type sameNames interface {
	Func(http.Request, http.Request)
}

type localer interface {
	Func(local)
}
