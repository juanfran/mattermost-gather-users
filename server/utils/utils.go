package utils

import (
	"math/rand"
	"time"
)

func ShuffleUsers(a []string) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
}

func Contains(slice []string, e string) bool {
	for _, a := range slice {
		if a == e {
			return true
		}
	}
	return false
}

func Remove(slice []string, toRemove string) []string {
	for i, v := range slice {
		if v == toRemove {
			slice = append(slice[:i], slice[i+1:]...)
			break
		}
	}

	return slice
}

func Prepend(data []string, item string) []string {
	data = append([]string{item}, data...)
	return data
  }
