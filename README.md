# Strongbox 8, a World of Warcraft addon manager

This is a rewrite of [Strongbox](https://github.com/ogri-la/strongbox) in [Go](https://go.dev) and [Tcl/TK](https://www.tcl-lang.org) and __is not stable, is not feature complete__ and  __is not safe to use in parallel with Strongbox 7__.

This repository __will be squashed and merged into regular [Strongbox](https://github.com/ogri-la/strongbox)__ once it exits alpha.

It runs on Linux and _Windows_.

It supports addons hosted by wowinterface.com, Github and Gitlab.

[Issues](./issues) and [Discussion](./discussions) are welcome. No code contributions, thank you.

## Major version changes

* Clojure replaced with Go, JavaFX replaced with Tcl/TK
* Mac support is dropped in favour of Windows
    - In order to compile for Mac I need macOS SDK headers that are only available if I agree to the XCode developer T&C's. I'm also not confident the resulting binary will be compatible with the GPL.
* To run the binaries you will need Tcl/TK installed, just like you needed Java to run the .jar files
* The `.AppImage` will continue to be the most convenient standalone binary for linux-amd64

## Second System Syndrome

I've rewritten Strongbox from Clojure and JavaFX into Go and Tcl/Tk. Clojure is still amazing, but:

I had designed myself into a corner by Strongbox 7

Small changes were having large effects and I wanted to be making large changes

I wasn't confident I could keep all the complexity in my head while developing

The time I have available to develop are a few hours on evenings and weekends and even finding that time is rare.

My enthusiasm for debugging run time errors and data problems was evaporating.

Java and Clojure are fast but Clojure's start up time is slooow.

Data structures I was working with had become a soupy mess.

`clojure.spec` and [Orchestra](https://github.com/jeaye/orchestra) are super powerful and were keeping me sane but they are slow during development and turned off outside of development. They aren't helpful at compile time.

The GUI was JavaFX and developed with the (often incomprehensibly) clever [cljfx](https://github.com/cljfx/cljfx). It felt like a black box between me and JavaFX. When I did manage to break through it I then had to contend with JavaFX, which is ... well, obtuse.

I do not use the REPL for development, I hate the idea of keeping all of that dirty state in my head and it underminding assumptions later. This makes me a poor Lisper ;)

I've finally developed an allergic reaction to clever code.

Clojure, for this small project, had become an impediment to dipping into development and to long term maintenance.

I have another project that loosely ties together 'providers' of 'services' in to a sort of 'data browser' that Strongbox is now helping to propel forwards. This is a classic [Second System Syndrome](https://en.wikipedia.org/wiki/Second-system_effect) and I'm not sorry ;)

I'm enjoying translating Clojure to Go. Getting the base system off the ground has been a faff and there is loads more to come, but I'm feeling it gel now and I'm able to re-model some of the lousiest aspects of Strongbox 7.0 - the data model especially.

Benefits of Strongbox 8:

* faster startup and a huge amount of room for speed improvements
* greater confidence during development because of static typing and analysis
* smaller, native binaries
* clearer addon data model
* shed a lot of baggage

Drawbacks of Strongbox 8.0

* shed a lot of baggage. Baggage is not bad! Strongbox 7.0 is hardened by experience with a comprehensive test suite.
* Tcl/Tk for the GUI. Love it or hate (I love it), it doesn't look 'modern' but it should feel more 'native'.
* code is definitely more verbose, but that is the Go way and I don't mind.
* Strongbox is now a module of a larger system so the interface will be less specific to addon management and more general.

## License

Copyright © 2025 Torkus

Distributed under the GNU Affero General Public Licence, version 3
