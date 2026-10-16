package main

import (
	"github.com/axiomzero-qed/WxMori/internal/cli"
	"os"
)

func main() { os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr)) }
