package main

import (
    "math/rand"
	"time"
)

var letters   = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var seed  	  = time.Now().UnixNano()
var generator = rand.New( rand.NewSource( 5 ) )

func init() {
	// Seed()
}

// Seed the generator for random number options.
// Example of argument default using optional arguments.
func Seed( pseudo ...bool ) {
	if len( pseudo ) > 1 && pseudo[0] || len( pseudo ) < 1 {
		generator = rand.New( rand.NewSource( seed ) )
	}
}

func RandSeq(n int) string {
    b := make([]rune, n)
    for i := range b {
        // b[i] = letters[rand.Intn(len(letters))]
        b[i] = letters[ generator.Intn( len( letters ) ) ]
    }
    return string(b)
}
