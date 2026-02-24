package config

// RunFirstTimeSetup is called when no config file is found.
// It delegates to tui.InitConfig to gather user input interactively
// and then saves the result. The actual implementation lives in internal/tui
// to keep the huh dependency out of the config package; this file just
// declares the hook that cmd/ wires up.
//
// The cmd layer calls tui.InitConfig() directly; this stub exists as
// documentation of the design intent.
