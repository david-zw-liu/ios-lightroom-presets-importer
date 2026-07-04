package main

func warningBanner() string {
	return "" +
		"========================== IMPORTANT ==========================\n" +
		" Fully close Lightroom on the device while volumes are mounted\n" +
		" (swipe it away in the app switcher). Reopen it after ejecting.\n" +
		"\n" +
		" Edits are written straight to the device. Eject a volume in\n" +
		" Finder when you are done — lrmount then unmounts the rest and\n" +
		" quits. Unplugging a cable ejects that device but keeps running.\n" +
		"\n" +
		" Note: presets written this way may appear only on this device\n" +
		" and may NOT sync to Creative Cloud.\n" +
		"===============================================================\n"
}
