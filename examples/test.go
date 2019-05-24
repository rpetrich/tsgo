package main

var foo int

func main() {
	var bar int
	ch := make(chan *int)
	ch <- &foo
	<-ch
	ch <- &bar
	<-ch
	go funcChannel(&foo)
	go func() {
		bar = 1
	}()
}

func funcChannel(arg *int) {
	ch := make(chan func())
	ch <- func() {
	}
	<-ch
}
