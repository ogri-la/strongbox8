# Strongbox 8.0: Second System Syndrome

I've rewritten Strongbox from Clojure into Go (Golang). Clojure is still amazing, go do more of it, but:

I had designed myself into a corner by Clojure 7.0

Small changes were having large effects and I wanted to be making large changes.

My confidence in keeping all of the complexity in my head while developing was proving difficult.

The time I have available to develop are a few hours on evenings and weekends and even finding that time is rare.

My enthusiasm for debugging run time errors and data problems was evaporating.

Java and Clojure are fast, but Clojure's start up time is slooow.

Data structures I was working with had become a sloppy mess of merging.

`clojure.spec` and [Orchestra](https://github.com/jeaye/orchestra) are super powerful and were keeping me sane, but they are slow during development and turned off outside of development. They aren't helpful at compile time.

The GUI was JavaFX and developed with the, often incomprehensibly clever, [cljfx](https://github.com/cljfx/cljfx). It felt like a blackbox between me and JavaFX and, should I manage to break through it, I then had to contend with JavaFX, which is ... well. Obtuse.

I do not use the REPL for development, I hate the idea of keeping all of that dirty state in my head and it underminding assumptions later.

I've finally developed an allergic reaction to clever code.

Clojure, for this small project, had become an impediment to dipping into development and to long term maintenance.

I have another project that loosely ties together 'providers' of 'services' in to a sort of 'data browser' that Strongbox is now helping to propel forwards. This is a classic [Second System Syndrome](https://en.wikipedia.org/wiki/Second-system_effect) and I'm not sorry ;)

I'm enjoying translating Clojure to Go. Getting the base system off the ground has been a faff and there is loads more to come, but I'm feeling it gel now and I'm able to re-model some of the lousiest aspects of Strongbox 7.0 - the data model especially.

Benefits of Strongbox 8.0:

* faster start up and a huge amount of room for speed improvements
* greater confidence during development because of static analysis
* native binaries for more systems
* clearer addon data model
* shed a lot of baggage.

Drawbacks of Strongbox 8.0

* shed a lot of baggage. Baggage is not bad! Strongbox 7.0 is hardened by experience with a comprehensive test suite.
* Tcl/Tk for the GUI. Love it or hate (I love it), it doesn't look 'modern' but it will feel 'native'.
* code is definitely more verbose, but that is the Go way and I don't mind.
* Strongbox is now a module of a larger system so the interface will be less specific to addon management and more general.

