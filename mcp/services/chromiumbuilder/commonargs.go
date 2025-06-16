package chromiumbuilder

import (
	"go.skia.org/infra/mcp/common"
)

// This file holds common.ToolArguments that are used by multiple tools.

var CommonToolArgumentBuilderGroup = common.ToolArgument{
	Name: "builder_group",
	Description: ("The builder group the builder will be a part of, e.g. chromium.fyi." +
		"This affects which file the builder will be added to as well as where it will show up " +
		"in the LUCI UI."),
	Required: true,
}

var CommonToolArgumentBuilderName = common.ToolArgument{
	Name: "builder_name",
	Description: ("The name of the new builder. It should be fairly descriptive, as this will " +
		"be the primary identifier that humans will see. Aspects that are commonly included are " +
		"the OS that is being compiled for as well as any uncommon traits. For example, if the builder " +
		"will be compiling with ASan enabled, it is good to include ASan in the name."),
	Required: true,
}

var CommonToolArgumentBuilderDescription = common.ToolArgument{
	Name: "builder_description",
	Description: ("A human-readable description of the builder that will be shown in the LUCI UI " +
		"when looking at the builder. This is where more in-depth information should go that does not " +
		"belong in the builder name. Supports HTML tags."),
	Required: true,
}

var CommonToolArgumentContactTeamEmail = common.ToolArgument{
	Name:        "contact_team_email",
	Description: "A valid email address for the team that will own the new builder.",
	Required:    true,
}

var CommonToolArgumentConsoleViewCategory = common.ToolArgument{
	Name: "console_view_category",
	Description: ("One or more categories used to group similar builders together. Each category is separated " +
		"by '|', with each level being progressively more nested. For example 'Linux|Asan' will " +
		"group the builder first with all other 'Linux' machines, then with all 'Asan' machines " +
		"are under 'Linux'."),
	Required: true,
}

var CommonToolArgumentTargetOs = common.ToolArgument{
	Name: "target_os",
	Description: ("The OS the builder is compiling for, e.g. 'Linux' or 'Android'. This is separate " +
		"from, but should be related to, the GN args that the builder will use for compilation."),
	Required:   true,
	EnumValues: []string{TargetOsAndroid, TargetOsLinux, TargetOsMac, TargetOsWin},
}

var CommonToolArgumentTargetArch = common.ToolArgument{
	Name: "target_arch",
	Description: ("The architecture the builder is compiling for, e.g. 'Arm'. This is separate " +
		"from, but should be related to, the GN args that the builder will use for compilation."),
	Required:   true,
	EnumValues: []string{TargetArchArm, TargetArchIntel},
}

var CommonToolArgumentTargetBits = common.ToolArgument{
	Name: "target_bits",
	Description: ("The target bitness the builder is compiling for, e.g. 32 or 64. This is separate " +
		"from, but should be related to, the GN args that the builder will use for compilation."),
	Required:     true,
	ArgumentType: common.NumberArgument,
	// Even though we reasonably only expect 32 and 64 as values, we cannot use
	// EnumValues since that only supports strings.
}

var CommonToolArgumentBuildConfig = common.ToolArgument{
	Name: "build_config",
	Description: ("The target config the builder is compiling for, e.g. 'Debug' or 'Release'. This is " +
		"separate from, but should be related to, the GN args that the builder will use for compilation."),
	Required:   true,
	EnumValues: []string{BuilderConfigDebug, BuilderConfigRelease},
}

var CommonToolArgumentTests = common.ToolArgument{
	Name: "tests",
	Description: ("The names of individual tests or bundles for the builder to compile and run. " +
		"Can be any number of individual tests from " +
		"https://source.chromium.org/chromium/chromium/src/+/main:infra/config/targets/tests.star " +
		"or bundles from " +
		"https://source.chromium.org/chromium/chromium/src/+/main:infra/config/targets/bundles.star. " +
		"At the current moment, only existing tests are supported, so new ones cannot be created as " +
		"part of this tool."),
	Required:     true,
	ArgumentType: common.ArrayArgument,
	ArraySchema:  map[string]any{"type": "string"},
}

var CommonToolArgumentSwarmingDimensions = common.ToolArgument{
	Name: "swarming_dimensions",
	Description: ("The names of Swarming mixins to use when triggering tests. " +
		"Can be any number of mixins from " +
		"https://source.chromium.org/chromium/chromium/src/+/main:infra/config/targets/mixins.star. " +
		"At the current moment, only existing mixins are supported, so new ones cannot be created as " +
		"part of this tool."),
	Required:     true,
	ArgumentType: common.ArrayArgument,
	ArraySchema:  map[string]any{"type": "string"},
}
