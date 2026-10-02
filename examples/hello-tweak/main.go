package main

import "gohome/ios"

func main() {
	ios.Hook("SpringBoard", "applicationDidFinishLaunching:", func(self ios.Object, original func(ios.Object, ios.Object), application ios.Object) {
		original(self, application)
		ios.Alert("gohome", "Твик написан на Go")
	})
}
