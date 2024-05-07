

## Usage

```go
package main

import (
	"context"
	"net/http"
	"time"
	
	"github.com/joerodriguez/multiflo"
)

func main() {
	flo := multiflo.New()
	defer flo.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(5*time.Second))
		defer cancel()

		// Claim a token for the request, block up to 5 seconds
		release, err := flo.Claim(ctx, map[string]string{
			"ip_address": r.RemoteAddr,
			"endpoint":   r.URL.Path,
		})

		// The claim was denied due to insufficient tokens
		if err != nil {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
			return
		}

		err = process(w, r)

		// Ensure the token is released when processing is complete
		// In addition to processing latency, also lets multiflo know 
		// if the request was successful or not
		release(err)
	})

	http.ListenAndServe(":8080", nil)
}
```
