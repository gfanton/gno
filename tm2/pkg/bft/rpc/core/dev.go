package core

import (
	"context"
	"os"
	"runtime/pprof"

	ctypes "github.com/gnolang/gno/tm2/pkg/bft/rpc/core/types"
)

// UnsafeFlushMempool removes all transactions from the mempool.
func UnsafeFlushMempool(ctx context.Context) (*ctypes.ResultUnsafeFlushMempool, error) {
	mempool.Flush()
	return &ctypes.ResultUnsafeFlushMempool{}, nil
}

var profFile *os.File

// UnsafeStartCPUProfiler starts a pprof profiler using the given filename.
func UnsafeStartCPUProfiler(ctx context.Context, filename string) (*ctypes.ResultUnsafeProfile, error) {
	var err error
	profFile, err = os.Create(filename)
	if err != nil {
		return nil, err
	}
	err = pprof.StartCPUProfile(profFile)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsafeProfile{}, nil
}

// UnsafeStopCPUProfiler stops the running pprof profiler.
func UnsafeStopCPUProfiler(ctx context.Context) (*ctypes.ResultUnsafeProfile, error) {
	pprof.StopCPUProfile()
	if err := profFile.Close(); err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsafeProfile{}, nil
}

// UnsafeWriteHeapProfile dumps a heap profile to the given filename.
func UnsafeWriteHeapProfile(ctx context.Context, filename string) (*ctypes.ResultUnsafeProfile, error) {
	memProfFile, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	if err := pprof.WriteHeapProfile(memProfFile); err != nil {
		return nil, err
	}
	if err := memProfFile.Close(); err != nil {
		return nil, err
	}

	return &ctypes.ResultUnsafeProfile{}, nil
}
