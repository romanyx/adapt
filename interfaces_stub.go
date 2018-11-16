package main

import (
	"io"
	"net/http"
)

type variadic interface {
	Func(...io.Reader)
}

type sameNames interface {
	Func(http.Request, http.Request)
}
