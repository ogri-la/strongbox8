# todo.md

this is my own scratchpad for keeping track of things. it gets truncated frequently.

see CHANGELOG.md for a more formal list of changes by release

## done

## headline: 8.0.0-alpha.2

* initial creation of strongbox config is borked
    - breaks this use case: 
        1. empty state
        2. create addons dir
        3. install addon
        4. ... nothing happens
    - problems:
        - settings are not created if parent dirs don't exist for new addons dir to be stored
            - done
        - creating a new addons dir doesn't automatically select it
            - done
        - installing an addon doesn't refresh the results
            - done
        - 'refresh' doesn't refresh the results so that items with children are now displayed
        - installing an addon doesn't switch to installed addons pane first
        - no directory selection on new addons dir
            - no, this is actually another bug when tracking expanded rows
                - I think the addonsdirs are being replaced and the rows in expanded_rows no longer match
                    - done
            - done
        - selected addon dir isn't obvious
            - done
        - creating multiple addons dirs shows the previous form
        - namespace 'ns' has a trailing slash
        - installing an addon when no addons dir selected fails without an error
* installing addon(s) from search results
    - switches to addons-dir tab
    - shows some sort of download progress
    - doesn't freeze UI
* 'update all' and per-addon updates seem to be borked
* columns don't have dummy values
    - created
    - updated
    - dirsize
    - combined-version
* tests exist
    - coverage is > 25%
* make a list of all features and their current implementation status
    - the first beta release will be feature complete with 7, even if it's ugly and unstable
* 'browse' button for addons dir
    - clicking it opens file browser
* 'properties' in context menu
    - select a thing, click 'properties', right pane opens with selected item properties

## todo bucket (no particular order)
* can't ctrl-backspace to delete 'words' in form text fields or search field
    - this is normal for tcl/tk apparently and will require extra work
* 'search' box on addons page isn't doing anything
* refresh 'f5' isn't working
* info message 'pruning zip files' should list number pruned
* don't create a 'logs' directory unless we're writing logs to fs
* review the logs being emitted:
    - debug: development only
    - info: displayed during normal usage of app
    - warn: important information that the user should be aware of _eventually_
    - error: important information that the user should be aware of _now_
* context menu, if addonsdir selected, don't offer to select it again
    - or at least don't do anything. I can see it reloads the addons dir
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

