package main

import "gohome/ios"

var taps int

func tapped() {
	taps++
	ios.Alert("gohome", "Нажатий: " + ios.Text(taps))
}

func main() {
	ios.Label("Привет из Go", 20, 60, 280, 40)
	ios.Button("Нажми меня", 20, 120, 280, 44, tapped)
}
