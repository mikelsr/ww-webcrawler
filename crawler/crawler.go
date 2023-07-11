package main

import (
	"context"
	"fmt"

	"github.com/wetware/ww/api/cluster"
	"github.com/wetware/ww/api/process"
	"github.com/wetware/ww/experiments/api/http"
	"github.com/wetware/ww/experiments/api/tools"
)

func main() {
	ctx := context.Background()

	// Bootstrap the required capabilites that were provided by the executor
	client, closer, err := BootstrapClient(ctx)
	defer closer.Close()
	if err != nil {
		panic(err)
	}

	// The inbox always contains a host, at least for now
	inbox := process.Inbox(client)
	open, release := inbox.Open(ctx, nil)
	defer release()

	host := cluster.Host(open.Content().AddRef())

	// The host points to its executor
	executor, err := executorFromHost(ctx, host)
	if err != nil {
		panic(err)
	}
	defer executor.Release()

	// The executor points to the experimental tools
	tools, err := toolsFromExecutor(ctx, executor)
	if err != nil {
		panic(err)
	}
	defer tools.Release()

	// The experimental tools have an http getter
	getter, err := getterFromTools(ctx, tools)
	if err != nil {
		panic(err)
	}
	defer getter.Release()

	res, err := get(ctx, getter, "https://notes.mikel.xyz")
	if err != nil {
		panic(err)
	}

	// The output will appear in the executor!
	fmt.Println(res)
}

func executorFromHost(ctx context.Context, host cluster.Host) (process.Executor, error) {
	f, _ := host.Executor(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return process.Executor{}, err
	}

	return res.Executor(), nil
}

func toolsFromExecutor(ctx context.Context, executor process.Executor) (tools.Tools, error) {
	f, _ := executor.Tools(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return tools.Tools{}, err
	}

	return res.Tools(), nil
}

func getterFromTools(ctx context.Context, tools tools.Tools) (http.HttpGetter, error) {
	f, _ := tools.Http(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return http.HttpGetter{}, err
	}

	return res.Getter(), nil
}
