package main

import (
	"fmt"

	cluster_api "github.com/wetware/ww/api/cluster"
)

func main() {
	client, closer, err := BootstrapClient()
	defer closer.Close()
	if err != nil {
		panic(err)
	}
	host := cluster_api.Host(client)
	fmt.Println(host)
}
