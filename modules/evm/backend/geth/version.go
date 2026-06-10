package geth

import "runtime/debug"

const GoEthereumModulePath = "github.com/ethereum/go-ethereum"

type DependencyInfo struct {
	Module  string `json:"module"`
	Version string `json:"version"`
	Sum     string `json:"sum,omitempty"`
	Replace string `json:"replace,omitempty"`
}

func GoEthereumDependencyInfo() DependencyInfo {
	info := DependencyInfo{Module: GoEthereumModulePath, Version: "unknown"}
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	for _, dependency := range buildInfo.Deps {
		if dependency.Path != GoEthereumModulePath {
			continue
		}
		info.Version = dependency.Version
		info.Sum = dependency.Sum
		if dependency.Replace != nil {
			info.Replace = dependency.Replace.Path
			if dependency.Replace.Version != "" {
				info.Replace += "@" + dependency.Replace.Version
			}
		}
		return info
	}
	return info
}
