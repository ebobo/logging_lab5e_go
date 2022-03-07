package main

import (
	"errors"

	"github.com/ebobo/utilities_go/greeting"
)

var ErrDBDoesNotExist = errors.New("database does not exist")

func main() {
	greeting.Foreword("Logging Package from Lab5e")

	lg.Info("log info")

	lg.Error("log error")

	// lg.Fatal("log fatal")
	lg.Infof("log infof %v", ErrDBDoesNotExist)

}
