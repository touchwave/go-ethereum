package txannounce

import "testing"

func TestWSServer(t *testing.T) {
	wss := newWS(":7856")
	err := wss.ListenAndServe()
	if err != nil {
		t.Error(err)
	}
}
