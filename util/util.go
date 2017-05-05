package util

import (

	"fmt"

	//TODO cleanup dependencies
	//"crypto/tls"
	"github.com/krallistic/kafka-operator/spec"
	//"net/http"
	//"time"
	//"encoding/json"


	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	//"k8s.io/client-go/kubernetes"
	//"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	appsv1Beta1 "k8s.io/client-go/pkg/apis/apps/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	log "github.com/Sirupsen/logrus"
//	"k8s.io/client-go/tools/cache"
//	"github.com/kubernetes/kubernetes/federation/pkg/federation-controller/util"

)

const (
	tprShortName = "kafka-cluster"
	tprSuffix = "incubator.test.com"
	tprFullName = tprShortName + "." + tprSuffix
	//API Name is used in the watch of the API, it defined as tprShorName, removal of -, and suffix s
	tprApiName = "kafkaclusters"
	tprVersion = "v1"

	tprName = "kafka.operator.com"
	tprEndpoint = "/apis/extensions/v1beta1/thirdpartyresources"
	defaultCPU = "1"
	defaultDisk = "100G"


)

var (
	getEndpoint = fmt.Sprintf("/apis/%s/%s/%s", tprSuffix, tprVersion, tprApiName)
	watchEndpoint = fmt.Sprintf("/apis/%s/%s/watch/%s", tprSuffix, tprVersion, tprApiName)
	logger = log.WithFields(log.Fields{
		"package": "util",
	})
)



type ClientUtil struct {
	KubernetesClient *k8sclient.Clientset
	MasterHost string
	DefaultOption metav1.GetOptions
	tprClient *rest.RESTClient
}

func EnrichSpecWithLogger(logger *log.Entry, cluster spec.KafkaCluster) *log.Entry {
	return logger.WithFields(log.Fields{"clusterName" : cluster.Metadata.Name, "namespace": cluster.Metadata.Name})
}

func New(kubeConfigFile, masterHost string) (*ClientUtil, error)  {

	// Create the client config. Use kubeconfig if given, otherwise assume in-cluster.
	config, err := buildConfig(kubeConfigFile)

	client, err := newKubeClient(kubeConfigFile)
	if err != nil {
		fmt.Println("Error, could not Init Kubernetes Client")
		return nil, err
	}
	tprClient, err := newTPRClient(config)
	if err != nil {
		fmt.Println("Error, could not Init KafkaCluster TPR Client")
		return nil, err
	}

	k := &ClientUtil{
		KubernetesClient: client,
		MasterHost: masterHost,
		tprClient:tprClient,
	}
	fmt.Println("Initilized k8s CLient")

	return k, nil

}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func configureClient(config *rest.Config) {
	groupversion := schema.GroupVersion{
		Group:   tprSuffix,
		Version: tprVersion,
	}

	config.GroupVersion = &groupversion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	schemeBuilder := runtime.NewSchemeBuilder(
		func(scheme *runtime.Scheme) error {
			scheme.AddKnownTypes(
				groupversion,
				&spec.KafkaCluster{},
				&spec.KafkaClusterList{},
			)
			return nil
		})
	metav1.AddToGroupVersion(scheme.Scheme, groupversion)
	schemeBuilder.AddToScheme(scheme.Scheme)
}

func newTPRClient(config *rest.Config) (*rest.RESTClient, error) {

	var tprconfig *rest.Config
	tprconfig = config
	configureClient(tprconfig)

	fmt.Println(tprconfig)

	tprclient, err := rest.RESTClientFor(tprconfig)
	if err != nil {
		panic(err)
	}

	// Fetch a list of our TPRs
	exampleList := spec.KafkaClusterList{}

	err = tprclient.Get().Resource(tprApiName).Do().Into(&exampleList)
	fmt.Printf("LIST: %#v\n", exampleList, err)
	if err != nil {
		logger.Warn("Error: ", err)
	}
	fmt.Printf("LIST: %#v\n", exampleList)
	//panic("exit")

	return tprclient, nil
}

//TODO refactor for config *rest.Config :)
func newKubeClient(kubeCfgFile string) (*k8sclient.Clientset, error) {

	var client *k8sclient.Clientset

	// Should we use in cluster or out of cluster config
	if len(kubeCfgFile) == 0 {
		fmt.Println("Using InCluster k8s without a kubeconfig")
		//Depends on k8s env and service account token set.
		cfg, err := rest.InClusterConfig()

		if err != nil {
			return nil, err
		}

		client, err = k8sclient.NewForConfig(cfg)

		if err != nil {
			return nil, err
		}
	} else {
		fmt.Println("Using OutOfCluster k8s config with kubeConfigFile: ", kubeCfgFile)
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeCfgFile)

		if err != nil {
			fmt.Println("Got error trying to create client: ", err)
			return nil, err
		}

		client, err = k8sclient.NewForConfig(cfg)

		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (c *ClientUtil) GetKafkaClusters() ([]spec.KafkaCluster, error) {
	methodLogger := logger.WithFields(log.Fields{"method": "GetKafkaClusters", })


	exampleList := spec.KafkaClusterList{}
	err := c.tprClient.Get().Resource(tprApiName).Do().Into(&exampleList)

	if err != nil {
		fmt.Println("Error while getting resonse from API: ", err)
		methodLogger.WithFields(log.Fields{
			"response":exampleList,
			"error": err,

		}).Error("Error response from API")
		return nil, err
	}
	methodLogger.WithFields(log.Fields{
		"response": exampleList,
	}).Info("KafkaCluster received")

	return exampleList.Items, nil
}
/// Create a the thirdparty ressource inside the Kubernetws Cluster
func (c *ClientUtil)CreateKubernetesThirdPartyResource() error  {
	methodLogger := logger.WithFields(log.Fields{"method": "CreateKubernetesThirdPartyResource",})
	tpr, err := c.KubernetesClient.ExtensionsV1beta1Client.ThirdPartyResources().Get(tprFullName, c.DefaultOption)
	if err != nil {
		if errors.IsNotFound(err) {
			methodLogger.WithFields(log.Fields{}).Info("No existing KafkaCluster TPR found, creating")

			tpr := &v1beta1.ThirdPartyResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: tprFullName,
				},
				Versions: []v1beta1.APIVersion{
					{Name: tprVersion},
				},
				Description: "Managed Apache Kafka clusters",
			}
			retVal, err := c.KubernetesClient.ThirdPartyResources().Create(tpr)
			if err != nil {
				methodLogger.WithFields(log.Fields{"response": err, }).Error("Error creating ThirdPartyRessources")
				panic(err)
			}
			methodLogger.WithFields(log.Fields{"response": retVal, }).Debug("Created KafkaCluster TPR")
		}
	} else {
		methodLogger.Info("KafkaCluster TPR already exist", tpr)
	}
	return nil
}

func Watch(client *rest.RESTClient, eventsChannel chan spec.KafkaClusterWatchEvent, signalChannel chan int) {
	methodLogger := logger.WithFields(log.Fields{"method": "Watch",})


	stop := make(chan struct{}, 1)
	source := cache.NewListWatchFromClient(
		client,
		tprApiName,
		v1.NamespaceAll,
		fields.Everything())

	store, controller := cache.NewInformer(
		source,

		&spec.KafkaCluster{},

		// resyncPeriod
		// Every resyncPeriod, all resources in the cache will retrigger events.
		// Set to 0 to disable the resync.
		0,

		// Your custom resource event handlers.
		cache.ResourceEventHandlerFuncs{
			// Takes a single argument of type interface{}.
			// Called on controller startup and when new resources are created.
			AddFunc: func(obj interface{}) {
				cluster := obj.(*spec.KafkaCluster)
				methodLogger.WithFields(log.Fields{"watchFunction": "ADDED"}).Info(spec.PrintCluster(cluster))
				var event spec.KafkaClusterWatchEvent
				//TODO
				event.Type = "ADDED"
				event.Object = *cluster
				fmt.Println(event)
				eventsChannel <- event
			},

			// Takes two arguments of type interface{}.
			// Called on resource update and every resyncPeriod on existing resources.
			UpdateFunc: func (old, new interface{}) {
				oldCluster := old.(*spec.KafkaCluster)
				newCluster := new.(*spec.KafkaCluster)
				fmt.Printf("UPDATED:\n  old: %s\n  new: %s\n", spec.PrintCluster(oldCluster), spec.PrintCluster(newCluster))
				var event spec.KafkaClusterWatchEvent
				//TODO refactor this.
				event.Type = "UPDATED"
				event.Object = *newCluster
				fmt.Println(event)
				eventsChannel <- event
			},

			// Takes a single argument of type interface{}.
			// Called on resource deletion.
			DeleteFunc: func (obj interface{}) {
				cluster := obj.(*spec.KafkaCluster)
				fmt.Println("delete", spec.PrintCluster(cluster))
				var event spec.KafkaClusterWatchEvent
				event.Type = "DELETE"
				event.Object = *cluster
				eventsChannel <- event
			},
		})

	// store can be used to List and Get
	// NEVER modify objects from the store. It's a read-only, local cache.
//	fmt.Println("listing examples from store:")
//	for _, obj := range store.List() {
//		example := obj.(*spec.KafkaCluster)
//
//		// This will likely be empty the first run, but may not
//		fmt.Printf("%#v\n", example)
//	}

	// the controller run starts the event processing loop
	go controller.Run(stop)
	fmt.Println(store)

	go func() {
		select {
		case <-signalChannel:
			fmt.Printf("received signal %#v, exiting...\n")
			close(stop)
		}
	}()

}


func (c *ClientUtil) MonitorKafkaEvents(eventsChannel chan spec.KafkaClusterWatchEvent, signalChannel chan int)  {
	methodLogger := logger.WithFields(log.Fields{"method": "MonitorKafkaEvents",})
	methodLogger.Info("Starting Watch")
	Watch(c.tprClient, eventsChannel, signalChannel)
}

func (c *ClientUtil) CreateStorage(cluster spec.KafkaClusterSpec) {
	//for every replica create storage image?
	//except for hostPath, emptyDir
	//let Sts create PV?

}

func (c *ClientUtil) CreateDirectBrokerService(cluster spec.KafkaCluster) error {

	brokerCount := cluster.Spec.BrokerCount
	fmt.Println("Creating N direkt broker SVCs, ", brokerCount)

	for  i := 0; i < 3 ; i++  {
		//TODO name dependend on cluster metadata
		name := "broker-" + string(i)
		fmt.Println("Creating Direct Broker SVC: ", i, name)
		svc, err := c.KubernetesClient.Services(cluster.Metadata.Namespace).Get(name, c.DefaultOption)
		if err != nil {
			return err
		}
		if len(svc.Name) == 0 {
			//Service dosnt exist, creating

			//TODO refactor creation ob object meta out,
			objectMeta := metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					"component": "kafka",
					"name":      name,
					"role": "data",
					"type": "service",
				},
			}
			service := &v1.Service{
				ObjectMeta: objectMeta,
				//TODO label selector
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"component": "kafka",
						"creator": "kafkaOperator",
						"role":      "data",
						"name": name,
					},
					Ports: []v1.ServicePort{
						v1.ServicePort{
							Name:     "broker",
							Port:     9092,
						},
					},
				},
			}
			_, err := c.KubernetesClient.Services(cluster.Metadata.Namespace).Create(service)
			if err != nil {
				fmt.Println("Error while creating direct service: ", err)
				return err
			}
			fmt.Println(service)

		}

	}



	return nil
}

//TODO refactor, into headless svc and direct svc
func (c *ClientUtil) CreateBrokerService(cluster spec.KafkaCluster, headless bool) error {
	//Check if already exists?
	name := cluster.Metadata.Name
	svc, err := c.KubernetesClient.Services(cluster.Metadata.Namespace).Get(name, c.DefaultOption)
	if err != nil {
		fmt.Println("error while talking to k8s api: ", err)
		//TODO better error handling, global retry module?

	}
	if len(svc.Name) == 0 {
		//Service dosnt exist, creating new.
		fmt.Println("Service dosnt exist, creating new")

		objectMeta := metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"component": "kafka",
				"name":      name,
				"role": "data",
				"type": "service",
			},
		}

		if headless == true {
			objectMeta.Labels = map[string]string{
				"service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
			}
			objectMeta.Name = name
		}

		service := &v1.Service{
			ObjectMeta: objectMeta,

			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"component": "kafka",
					"creator": "kafkaOperator",
					"role":      "data",
					"name": name,
				},
				Ports: []v1.ServicePort{
					v1.ServicePort{
						Name:     "broker",
						Port:     9092,
					},
				},
				ClusterIP: "None",
			},
		}
		_, err := c.KubernetesClient.Services(cluster.Metadata.Namespace).Create(service)
		if err != nil {
			fmt.Println("Error while creating Service: ", err)
		}
		fmt.Println(service)
	} else {
		//Service exist
		fmt.Println("Headless Broker SVC already exists: ", svc)
		//TODO maybe check for correct service?
	}

	return nil
}


//TODO caching of the STS
func (c *ClientUtil) BrokerStatefulSetExist(cluster spec.KafkaCluster) bool {

	statefulSet, err := c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Get(cluster.Metadata.Name, c.DefaultOption)
	if err != nil ||  len(statefulSet.Name) == 0 {
		return false
	}
	return true
}

func (c *ClientUtil) BrokerStSImageUpdate(cluster spec.KafkaCluster) bool {
	kafkaClusterSpec := cluster.Spec

	statefulSet, err := c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Get(cluster.Metadata.Name, c.DefaultOption)
	if err != nil {
		fmt.Println("TODO error?")
	}
	//TODO multiple Containers

	if (len(statefulSet.Spec.ServiceName) == 0) && (statefulSet.Spec.Template.Spec.Containers[0].Image != kafkaClusterSpec.Image) {
		return true
	}
	return false
}

func (c *ClientUtil) BrokerStSUpsize(cluster spec.KafkaCluster) bool {
	statefulSet, _ := c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Get(cluster.Metadata.Name, c.DefaultOption)
	return *statefulSet.Spec.Replicas < cluster.Spec.BrokerCount
}

func (c *ClientUtil) BrokerStSDownsize(cluster spec.KafkaCluster) bool {
	statefulSet, _ := c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Get(cluster.Metadata.Name, c.DefaultOption)
	return *statefulSet.Spec.Replicas > cluster.Spec.BrokerCount
}

func (c *ClientUtil) createStsFromSpec(cluster spec.KafkaCluster) *appsv1Beta1.StatefulSet {
	methodLogger := logger.WithFields(log.Fields{"method": "createStsFromSpec",})
	methodLogger = EnrichSpecWithLogger(methodLogger, cluster)

	name := cluster.Metadata.Name
	replicas := cluster.Spec.BrokerCount
	image := cluster.Spec.Image

	//TODO error handling, default value?
	cpus, err := resource.ParseQuantity(cluster.Spec.Resources.CPU)
	if err != nil {
		cpus, _ = resource.ParseQuantity(defaultCPU)
	}
	diskSpace, err := resource.ParseQuantity(cluster.Spec.Resources.DiskSpace)
	if err != nil {
		diskSpace, _ = resource.ParseQuantity(defaultDisk)
	}

	statefulSet := &appsv1Beta1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"component": "kafka",
				"creator": "kafkaOperator",
				"role":      "data",
				"name": name,
			},
		},
		Spec: appsv1Beta1.StatefulSetSpec{
			Replicas: &replicas,

			ServiceName: cluster.Metadata.Name,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"component": "kafka",
						"creator": "kafkaOperator",
						"role": "data",
						"name": name,
					},
				},
				Spec:v1.PodSpec{
					InitContainers: []v1.Container{
						v1.Container{
							Name: "labeler",
							Image: "devth/k8s-labeler", //TODO fullName, config
							Env: []v1.EnvVar{
								v1.EnvVar{
									Name: "KUBE_NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								v1.EnvVar{
									Name: "KUBE_LABEL_hostname",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
						},
						v1.Container{
							Name: "zookeeper-ready",
							Image: "busybox", //TODO full Name, config
							Command: []string{"sh", "-c", fmt.Sprintf(
								"until nslookup %s; do echo waiting for myservice; sleep 2; done;",
								cluster.Spec.ZookeeperConnect)},
						},
					},

					Containers: []v1.Container{
						v1.Container{
							Name: "kafka",
							Image: image,
							//TODO String replace operator etc
							Command: []string{"/bin/bash",
									  "-c",
								fmt.Sprintf("export KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://$(hostname).%s.$(NAMESPACE).svc.cluster.local:9092; \n" +
									"set -ex\n" +
									"[[ `hostname` =~ -([0-9]+)$ ]] || exit 1\n" +
									"export KAFKA_BROKER_ID=${BASH_REMATCH[1]}\n" +
									"/etc/confluent/docker/run",name),
							},
							Env: []v1.EnvVar{
								v1.EnvVar{
									Name: "NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								v1.EnvVar{
									Name:  "KAFKA_ZOOKEEPER_CONNECT",
									Value: cluster.Spec.ZookeeperConnect,
								},
							},
							Ports: []v1.ContainerPort{
								v1.ContainerPort{
									Name: "kafka",
									ContainerPort: 9092,
								},
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU: cpus,
								},
							},

						},
					},
				},


			},
			VolumeClaimTemplates: []v1.PersistentVolumeClaim{
				v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kafka-data",
						Annotations: map[string]string{
							//TODO make storageClass Optinal
							//"volume.beta.kubernetes.io/storage-class": "anything",
						},
					},
					Spec: v1.PersistentVolumeClaimSpec{
						AccessModes: []v1.PersistentVolumeAccessMode{
							v1.ReadWriteOnce,
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceStorage: diskSpace,
							},
						},
					},
				},
			},
		},
	}
	return statefulSet
}


func (c *ClientUtil) UpsizeBrokerStS(cluster spec.KafkaCluster) error {

	statefulSet, err := c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Get(cluster.Metadata.Name, c.DefaultOption)
	if err != nil ||  len(statefulSet.Name) == 0 {
		return err
	}
	statefulSet.Spec.Replicas = &cluster.Spec.BrokerCount
	_ ,err = c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Update(statefulSet)

	if err != nil {
		fmt.Println("Error while updating Broker Count")
	}

	return err
}

func (c *ClientUtil) UpdateBrokerImage(cluster spec.KafkaCluster) error {
	statefulSet, err := c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Get(cluster.Metadata.Name, c.DefaultOption)
	if err != nil ||  len(statefulSet.Name) == 0 {
		return err
	}
	statefulSet.Spec.Template.Spec.Containers[0].Image = cluster.Spec.Image

	_ ,err = c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Update(statefulSet)

	if err != nil {
		fmt.Println("Error while updating Broker Count")
		return err
	}

	return nil
}

func (c *ClientUtil) CreatePersistentVolumes(cluster spec.KafkaCluster) error{
	fmt.Println("Creating Persistent Volumes for KafkaCluster")

	pv, err := c.KubernetesClient.PersistentVolumes().Get("testpv-1", c.DefaultOption)
	if err != nil  {
		return err
	}
	if len(pv.Name) == 0 {
		fmt.Println("PersistentVolume dosnt exist, creating")
		new_pv := v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:"test-1",
			},
			Spec: v1.PersistentVolumeSpec{
				AccessModes:[]v1.PersistentVolumeAccessMode{
					v1.ReadWriteOnce,
				},
				//Capacity: Reso

			},

		}
		fmt.Println(new_pv)
	}

	return nil

}

func (c *ClientUtil) DeleteKafkaCluster(cluster spec.KafkaCluster) error {

	var gracePeriod int64
	gracePeriod = 10
	//var orphan bool
	//orphan = true
	deleteOption := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}

	//Delete Services
	err := c.KubernetesClient.Services(cluster.Metadata.Namespace).Delete(cluster.Metadata.Name, &deleteOption)
	if err != nil {
		fmt.Println("Error while deleting Broker Service: ", err)
	}

	statefulSet, err := c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Get(cluster.Metadata.Name, c.DefaultOption)//Scaling Replicas down to Zero
	if (len(statefulSet.Name) == 0 ) && ( err != nil) {
		fmt.Println("Error while getting StS from k8s: ", err)
	}

	var replicas int32
	replicas = 0
	statefulSet.Spec.Replicas = &replicas

	_, err = c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Update(statefulSet)
	if err != nil {
		fmt.Println("Error while scaling down Broker Sts: ", err)
	}
	//Delete Volumes
	//TODO when volumes are implemented


	//TODO better Error handling
	return err
}


func (c *ClientUtil) CreateBrokerStatefulSet(cluster spec.KafkaCluster) error {

	//Check if sts with Name already exists
	statefulSet, err := c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Get(cluster.Metadata.Name, c.DefaultOption)

	if err != nil {
		fmt.Println("Error get sts")
	}
	if len(statefulSet.Name) == 0 {
		fmt.Println("STS dosnt exist, creating")

		statefulSet := c.createStsFromSpec(cluster)

		fmt.Println(statefulSet)
		_, err := c.KubernetesClient.StatefulSets(cluster.Metadata.Namespace).Create(statefulSet)
		if err != nil {
			fmt.Println("Error while creating StatefulSet: ", err)
			return err
		}
	} else {
		fmt.Println("STS already exist.", statefulSet)
	}
	return nil
}