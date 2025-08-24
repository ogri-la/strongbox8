# todo.md

this is my own scratchpad for keeping track of things. it gets truncated frequently.

see CHANGELOG.md for a more formal list of changes by release

## done

## headline: 8.0.0-alpha.2

* 'update all' and per-addon updates seem to be borked
* columns don't have dummy values
    - created
    - updated
    - dirsize
    - combined-version
* tests exist
    - coverage is > 25%
* installing addon(s) from search results
    - switches to addons-dir tab
    - shows some sort of download progress
    - doesn't freeze UI
* make a list of all features and their current implementation status
    - the first beta release will be feature complete with 7, even if it's ugly and unstable
* 'browse' button for addons dir
    - clicking it opens file browser
* 'properties' in context menu
    - select a thing, click 'properties', right pane opens with selected item properties

## todo bucket (no particular order)

* selecting a new addons dir collapses others
* collapse rows just once after inserting all
    - rather than per-batch

* tooltips
    - WoW column
    - for headers as well
* double clicking expands a row
* arrow has it's own column
* columns are sized to fit
* per-tab customisable filterable results
    - for example, ctrl-f on addons-dir tab will search folder name and selected game track
* column sizing
* clickable things in results
    - urls
    - tags
    - 
* auto-pagination for long lists of things

* task runners with progress bars

* styling, what we have is *basic*
    - per-row styling as well

* "focus" option for search results
    - creates a new view with just the selected results

* "expand all" option for search results
    - expands all children that are not lazy

* properly collapsible details pane
    - it only collapses as much as the contents inside it
    - if resized it won't collapse at all

* 'exploded' views
    - ask a thing to yield a list of Results
    - different from 'children'
        - for example, an 'addon' may have multiple '.toc' files, but, it also may yield:
            - a set of related directories
            - a url
            - ...

