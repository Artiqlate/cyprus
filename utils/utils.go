package utils

import (
	"fmt"
	"log"

	"github.com/vmihailenco/msgpack/v5"
)

func ValidateDecoder(decoder *msgpack.Decoder) error {
	dataArrLen, arrLenErr := decoder.DecodeArrayLen()
	if arrLenErr != nil {
		return arrLenErr
	}
	if dataArrLen < 2 {
		return fmt.Errorf("RPC missing payload")
	}
	return nil
}

func LogFunc(modName string, fstr string, i ...interface{}) {
	if i != nil {
		log.Printf("%s\t: "+fmt.Sprintf(fstr, i...), modName)
	} else {
		log.Printf("%s\t: "+fstr, modName)
	}
}
