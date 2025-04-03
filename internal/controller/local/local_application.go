package local

type LocalApplication struct {
}

func LoadFromKubernetes() LocalApplication {
	return LocalApplication{}
}
