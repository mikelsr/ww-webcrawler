package main

import (
	"context"
	"fmt"

	cluster_api "github.com/wetware/ww/api/cluster"
	process_api "github.com/wetware/ww/api/process"
	http_api "github.com/wetware/ww/experiments/api/http"
	tools_api "github.com/wetware/ww/experiments/api/tools"
)

func main() {
	ctx := context.Background()
	client, closer, err := BootstrapClient(ctx)
	defer closer.Close()
	if err != nil {
		panic(err)
	}

	inbox := process_api.Inbox(client)
	open, release := inbox.Open(ctx, nil)
	defer release()

	host := cluster_api.Host(open.Content().AddRef())

	executor, err := executorFromHost(ctx, host)
	if err != nil {
		panic(err)
	}
	defer executor.Release()

	tools, err := toolsFromExecutor(ctx, executor)
	if err != nil {
		panic(err)
	}
	defer tools.Release()

	getter, err := getterFromTools(ctx, tools)
	if err != nil {
		panic(err)
	}
	defer getter.Release()

	res, err := get(ctx, getter, "https://notes.mikel.xyz")
	if err != nil {
		panic(err)
	}

	fmt.Println(res)
}

func executorFromHost(ctx context.Context, host cluster_api.Host) (process_api.Executor, error) {
	f, _ := host.Executor(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return process_api.Executor{}, err
	}

	return res.Executor(), nil
}

func toolsFromExecutor(ctx context.Context, executor process_api.Executor) (tools_api.Tools, error) {
	f, _ := executor.Tools(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return tools_api.Tools{}, err
	}

	return res.Tools(), nil
}

func getterFromTools(ctx context.Context, tools tools_api.Tools) (http_api.HttpGetter, error) {
	f, _ := tools.Http(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return http_api.HttpGetter{}, err
	}

	return res.Getter(), nil
}
