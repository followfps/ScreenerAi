package freeqwenproxy

import "strings"

var canonicalModels = []string{
	"qwen3-max",
	"qwen3-vl-plus",
	"qwen3-coder-plus",
	"qwen3-omni-flash",
	"qwen-plus-2025-09-11",
	"qwen3-235b-a22b",
	"qwen3-30b-a3b",
	"qwen3-coder-30b-a3b-instruct",
	"qwen-max-latest",
	"qwen-plus-2025-01-25",
	"qwq-32b",
	"qwen-turbo-2025-02-11",
	"qwen2.5-omni-7b",
	"qvq-72b-preview-0310",
	"qwen2.5-vl-32b-instruct",
	"qwen2.5-14b-instruct-1m",
	"qwen2.5-coder-32b-instruct",
	"qwen2.5-72b-instruct",
}

var aliasGroups = map[string][]string{
	"qwen3-max": {
		"qwen-max",
		"Qwen3-Max",
		"Qwen3-Maximum",
		"qwen3-max-preview",
		"Qwen3-Max-Preview",
	},
	"qwen3-vl-plus": {
		"qwen-vl",
		"qwen-vl-plus",
		"qwen-vl-plus-latest",
		"qwen-vl-max",
		"qwen-vl-max-latest",
		"Qwen3-VL-235B-A22B",
		"qwen3-vl-235b-a22b",
	},
	"qwen3-coder-plus": {
		"qwen3-coder",
		"qwen-coder-plus",
		"qwen-coder-plus-latest",
		"Qwen3-Coder-Plus",
		"qwen2.5-coder-3b-instruct",
		"qwen2.5-coder-1.5b-instruct",
		"qwen2.5-coder-0.5b-instruct",
		"Qwen3-Coder",
	},
	"qwen3-omni-flash": {
		"qwen3-omni",
		"qwen3-omni-latest",
		"Qwen3-omni-flash",
		"Qwen3-Omni-Flash",
		"Qwen3-Omni",
	},
	"qwen-plus-2025-09-11": {
		"qwen-plus",
		"qwen-plus-latest",
		"Qwen3-Next",
		"Qwen3-Next-80B-A3B",
		"Qwen3-Next-80B-A3Bб",
		"qwen3-next",
		"qwen3-next-80b-a3b",
	},
	"qwen3-235b-a22b": {
		"qwen3",
		"qwen-3",
		"qwen3-235b",
		"Qwen3-235B-A22B",
		"Qwen3-235B-A22B-2507",
		"qwen3-235b-a22b-2507",
	},
	"qwen3-30b-a3b": {
		"qwen3-plus",
		"qwen3-30b",
		"Qwen3-30B-A3B",
		"Qwen3-30B-A3B-2507",
		"qwen3-30b-a3b-2507",
	},
	"qwen3-coder-30b-a3b-instruct": {
		"qwen3-coder-flash",
		"Qwen3-Coder-Flash",
		"qwen3-coder-30b",
		"Qwen3-Coder-30B-A3B-Instruct",
	},
	"qwen-max-latest": {
		"Qwen2.5-Max",
		"qwen2.5-max",
	},
	"qwen-plus-2025-01-25": {
		"Qwen2.5-Plus",
		"qwen2.5-plus",
	},
	"qwq-32b": {
		"qwq",
		"QwQ-32B",
		"qwq-32b-preview",
	},
	"qwen-turbo-2025-02-11": {
		"qwen-turbo",
		"qwen-turbo-latest",
		"Qwen2.5-Turbo",
	},
	"qwen2.5-omni-7b": {
		"qwen2.5-omni",
		"Qwen2.5-Omni-7B",
		"qwen-omni-7b",
	},
	"qvq-72b-preview-0310": {
		"qvq",
		"QVQ-Max",
		"qvq-72b",
	},
	"qwen2.5-vl-32b-instruct": {
		"qwen2.5-vl",
		"Qwen2.5-VL-32B-Instruct",
	},
	"qwen2.5-14b-instruct-1m": {
		"qwen2.5-14b",
		"qwen2.5-coder-14b-instruct",
		"Qwen2.5-14B-Instruct-1M",
	},
	"qwen2.5-coder-32b-instruct": {
		"qwen2.5-coder",
		"qwen2.5-coder-plus",
		"Qwen2.5-Coder-32B-Instruct",
	},
	"qwen2.5-72b-instruct": {
		"qwen2.5-72b",
		"Qwen2.5-72B-Instruct",
	},
}

var modelMapping = buildModelMapping()

func buildModelMapping() map[string]string {
	m := make(map[string]string, 256)
	canonSet := make(map[string]struct{}, len(canonicalModels))
	for _, v := range canonicalModels {
		canonSet[v] = struct{}{}
		m[v] = v
	}
	for target, aliases := range aliasGroups {
		if _, ok := canonSet[target]; !ok {
			continue
		}
		for _, a := range aliases {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			m[a] = target
		}
	}
	return m
}

func GetMappedModel(requestedModel, defaultModel string) string {
	if strings.TrimSpace(defaultModel) == "" {
		defaultModel = "qwen-max-latest"
	}
	req := strings.TrimSpace(requestedModel)
	if req == "" {
		return defaultModel
	}
	if target, ok := modelMapping[req]; ok && strings.TrimSpace(target) != "" {
		return target
	}
	return defaultModel
}

