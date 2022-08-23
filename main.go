package main

func main() {
	dialer := NewSocksDialer("tcp", "127.0.0.1:1080")
	listener := NewListener("tcp", ":1081", &HTTPHandler{Dialer: dialer})

	listener.Serve()

	quit := make(chan bool)
	<-quit
}
