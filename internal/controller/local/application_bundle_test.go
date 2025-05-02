package local

import (
	"testing"

	"hiro.io/anyapplication/internal/controller/fixture"
)

func TestApplicationBundle(t *testing.T) {
	expected := fixture.LoadJSONFixture[ApplicationBundle](t, "application_bundle.json")
	serialized, _ := expected.Serialize()
	_ = fixture.SaveStringFixture("application_bundle_clean.json", serialized)
	// raw := fixture.LoadStringFixture(t, "application_bundle_clean.json")
	// actual, _ := Deserialize(serialized)
	// if !reflect.DeepEqual(actual, expected) {
	// 	t.Error("Serialization/Deserialization error")
	// }
}

// func TestLoadApplicationBundle(t *testing.T) {
// 	kubeconfig := "env/dev/target/merged-kubeconfig.yaml"
// 	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// Create a Kubernetes client
// 	k8sClient, err := client.New(config, client.Options{})
// 	if err != nil {
// 		panic(err)
// 	}
// 	applicationSpec := &hirov1.AnyApplicationSpec{
// 		Application: hirov1.ApplicationMatcherSpec{
// 			ResourceSelector: map[string]string{
// 				"k8s-app": "kube-dns",
// 			},
// 		},
// 	}

// 	bundle, err := LoadApplicationBundle(context.TODO(), k8sClient, &applicationSpec.Application)
// 	t.Logf("%s", err)
// 	serialized, _ := bundle.Serialize()
// 	fixture.SaveStringFixture(t, "application_bundle.json", serialized)

// 	cleanBundle := bundle.CleanResources()
// 	result, err := cleanBundle.Serialize()
// 	t.Logf("%s", err)

// 	new, err := Deserialize(result)
// 	if err != nil {
// 		t.Fatalf("failed to deserialize JSON fixture %s: %v", result, err)
// 	}
// 	serializedNew, _ := new.Serialize()
// 	t.Logf("Clientset created successfully: %s", serializedNew)
// }
