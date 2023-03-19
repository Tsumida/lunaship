package res

import v1 "github.com/tsumida/lunaship/api/v1"

var EXAMPLE_LIST_MACHINES *v1.ListMachineResult = &v1.ListMachineResult{
	Total: 2,
	Machines: []*v1.Machine{
		{
			UniqId:      "MACHINE_JSKNKAJSD71283",
			MachineName: "server-A",
			LanIps: []string{
				"10.20.30.40",
			},
			WanIps: []string{},
		},
		{
			UniqId:      "MACHINE_JSKNKAJSD71284",
			MachineName: "server-B",
			LanIps: []string{
				"10.20.30.41",
				"10.20.30.42",
			},
			WanIps: []string{
				"43.222.111.1",
			},
		},
	},
}
