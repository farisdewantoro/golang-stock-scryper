package utils

import (
	"log"
	"time"
)

func TimeNowWIB() time.Time {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Fatal("Failed to load location", err)
	}
	return time.Now().In(loc)
}
