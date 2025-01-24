package core

func DirArgDef() ArgDef {
	return ArgDef{
		ID:     "dir",
		Label:  "Directory",
		Parser: ParseStringAsPath,
		ValidatorList: []PredicateFn{
			IsDirValidator,
		},
	}
}

func ConfirmYesArgDef() ArgDef {
	return ArgDef{
		ID:      "confirm",
		Label:   "Are you sure?",
		Default: "yes",
		Parser:  ParseYesNoStringAsBool,
	}
}
