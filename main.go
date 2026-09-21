package main

import (
	"fmt"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/davidmuller5273-boop/safe/cmd"
)

func main() {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Fprintln(os.Stderr, "加载北京时区失败:", err)
		os.Exit(1)
	}
	time.Local = location

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
