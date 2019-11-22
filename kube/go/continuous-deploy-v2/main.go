// continuous-deploy-v2 monitors pubsub events for the GCP Container Builder and
// pushes updated images when successful images are built.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
	//"strings"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/continuous_deploy"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/httputils"
	//"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitauth"
	"go.skia.org/infra/go/gitiles"
	//"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	//cloudbuild "google.golang.org/api/cloudbuild/v1"
	firestore_api "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TODO(rmistry): same app. one for only tagging.. other for tagging+deploying

// flags
var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	project  = flag.String("project", "skia-public", "The GCE project name.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")

	deployImages  = common.NewMultiStringFlag("deploy_image", nil, "Docker image that the continuous deploy app should deploy when it's docker image is built, if it is newer than the last encountered hash.")
	tagProdImages = common.NewMultiStringFlag("tag_prod_image", nil, "Docker image that the continuous deploy app should tag as 'prod' if it is newer than the last encountered hash.")

	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'continuous-deploy-v2'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
)

var (
	parseImageName *regexp.Regexp
)

const (
	// For accessing Firestore.
	DEFAULT_ATTEMPTS      = 3
	PUT_SINGLE_TIMEOUT    = 10 * time.Second
	DELETE_SINGLE_TIMEOUT = 10 * time.Second
)

func Init() {
	parseImageName = regexp.MustCompile("^gcr.io/" + *project + "/([^:]+).*$")
}

// baseImageName returns "fiddler" from "gcr.io/skia-public/fiddler:foo".
//
// If the image name doesn't start with "gcr.io" and the project name then "" is returned.
func baseImageName(s string) string {
	matches := parseImageName.FindStringSubmatch(s)
	if len(matches) != 2 {
		return ""
	} else {
		return matches[1]
	}
}

// TODO(rmistry): Update this doc
// imagesFromInfo parses the incoming PubSub Data 'b' as JSON and then returns
// the full image names of all the images that match 'shortImageNames'.
func imageAndTagFromInfo(shortImageNames []string, buildInfo continuous_deploy.BuildInfo) (string, string) {
	sklog.Infof("ImageName: %s", buildInfo.ImageName)
	sklog.Infof("Tag: %s", buildInfo.Tag)
	// Is this one of the images we are pushing?
	for _, name := range shortImageNames {
		if baseImageName(buildInfo.ImageName) == name {
			return buildInfo.ImageName, buildInfo.Tag
		}
	}
	return "", ""
}

func imageTagId(imageName, tag string) string {
	return fmt.Sprintf("%s#%s#%s", imageName, tag, firestore.FixTimestamp(time.Now()).Format(util.SAFE_TIMESTAMP_FORMAT))
}

func main() {
	common.InitWithMust(
		"continuous-deploy-v2",
		common.PrometheusOpt(promPort),
	)

	if *deployImages == nil && *tagProdImages == nil {
		sklog.Fatal("Must pass in atleast one of --deploy_image and --tag_prod_image")
	}
	if *fsNamespace == "" {
		sklog.Fatalf("--fs_namespace must be set")
	}

	Init()
	//sklog.Infof("Pushing to: %v", flag.Args())
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local, pubsub.ScopePubSub, "https://www.googleapis.com/auth/gerritcodereview")
	if err != nil {
		sklog.Fatal(err)
	}
	if !*local {
		_, err := gitauth.New(ts, "/tmp/git-cookie", true, "skia-continuous-deploy@skia-public.iam.gserviceaccount.com")
		if err != nil {
			sklog.Fatal(err)
		}
	}
	client, err := pubsub.NewClient(ctx, *project, option.WithTokenSource(ts))
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic(continuous_deploy.TOPIC)
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	subName := fmt.Sprintf("%s-%s", continuous_deploy.TOPIC, hostname)
	sub := client.Subscription(subName)
	ok, err := sub.Exists(ctx)
	if err != nil {
		sklog.Fatalf("Failed checking subscription existence: %s", err)
	}
	if !ok {
		sub, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: topic,
		})
		if err != nil {
			sklog.Fatalf("Failed creating subscription: %s", err)
		}
	}
	pushk := "/usr/local/bin/pushk"
	if *local {
		pushk = "pushk"
	}
	fmt.Println(pushk)

	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, *fsProjectID, "continuous-deploy-v2", *fsNamespace, ts)
	if err != nil {
		sklog.Fatalf("Could not init firestore: %s", err)
	}
	col := fsClient.Collection("testImageName")
	//id := imageTagId("testImageName", "testtag")
	id := "testImageName"
	b := &continuous_deploy.BuildInfo{
		ImageName: "testImageName",
		Tag:       "testtag2",
	}

	// This is to query.
	var fromB continuous_deploy.BuildInfo

	docs := []*firestore_api.DocumentSnapshot{}
	iter := col.Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			sklog.Fatal(err)
		}
		//fmt.Println("READING READING")
		//fmt.Println(doc)
		//fmt.Println(doc.Data)
		docs = append(docs, doc)
		// Test to see if
	}

	// replace image doc if:
	// * hash is newer
	// * first time we have encountered this image.
	addImageDoc := false
	if len(docs) > 1 {
		sklog.Fatalf("Something went wrong here...")
	} else if len(docs) == 1 {
		if err := docs[0].DataTo(&fromB); err != nil {
			sklog.Fatal(err)
		}
		// Now test here to see if the hash is newer or not...
		// if newer then set addImageDoc = true
		// and delete!!!
		addImageDoc = true

		// Delete it
		_, deleteErr := fsClient.Delete(ctx, col.Doc(id), DEFAULT_ATTEMPTS, DELETE_SINGLE_TIMEOUT)
		if deleteErr != nil {
			sklog.Fatalf("Could not delete %s in firestore: %s", id, deleteErr)
		}

		// If newer then delete and replace.
	} else {
		// first time we are seeing this image
		addImageDoc = true
	}

	if addImageDoc {
		// This is to add.
		_, createErr := fsClient.Create(ctx, col.Doc(id), b, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT)
		if st, ok := status.FromError(createErr); ok && st.Code() == codes.AlreadyExists {
			sklog.Fatalf("ID: %s already exists in firestore: %s", id, createErr)
		}
		if createErr != nil {
			sklog.Fatalf("Could not create %s in firestore: %s", id, createErr)
		}
	}

	fmt.Println(fromB.ImageName)
	fmt.Println(fromB.Tag)

	// Do the gitiles thingy now... to test with!!!!!

	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	r := gitiles.NewRepo(common.REPO_SKIA, httpClient)
	log, err := r.LogLinear(ctx, "6ec1b39f0c1bc145048a2de92553117f899cdbed", "1792b19485cacb0c950a3c0ea0aab763a8d43176")
	if err != nil {
		sklog.Fatalf("Could not query gitiles of %s: %s", common.REPO_SKIA, err)
	}
	fmt.Println("RESULT")
	fmt.Println(log)
	log2, err := r.LogLinear(ctx, "1792b19485cacb0c950a3c0ea0aab763a8d43176", "6ec1b39f0c1bc145048a2de92553117f899cdbed")
	if err != nil {
		sklog.Fatalf("Could not query gitiles of %s: %s", common.REPO_SKIA, err)
	}
	fmt.Println("RESULT")
	fmt.Println(log2)

	// Do the docker tests now.

	pullCmd := fmt.Sprintf("docker pull %s", "gcr.io/skia-public/infra-v2:test5")
	_, pullErr := exec.RunSimple(ctx, pullCmd)
	if pullErr != nil {
		sklog.Fatalf("Error running docker pull: %s", pullErr)
	}

	tagCmd := fmt.Sprintf("docker tag %s %s:%s", "gcr.io/skia-public/infra-v2:test5", "gcr.io/skia-public/infra-v2", "test6")
	_, tagErr := exec.RunSimple(ctx, tagCmd)
	if tagErr != nil {
		sklog.Fatalf("Error running docker tag: %s", tagErr)
	}

	pushCmd := fmt.Sprintf("docker push %s:%s", "gcr.io/skia-public/infra-v2", "test6")
	_, pushErr := exec.RunSimple(ctx, pushCmd)
	if pushErr != nil {
		sklog.Fatalf("Error running docker push: %s", pushErr)
	}

	// docker pull gcr.io/skia-public/infra-v2:test2
	// docker tag gcr.io/skia-public/infra-v2:test2 gcr.io/skia-public/infra-v2:test4
	// docker push gcr.io/skia-public/infra-v2:test4

	fmt.Println("JUST TESTING SO DONE!")
	return

	//colSnap, err := col.Get(ctx)
	//if err != nil {
	//	sklog.Fatalf("Could not get from firestore contents of %s: %s", col, err)
	//}
	//var fromB continuous_deploy.BuildInfo
	//if err := colSnap.DataTo(&fromB); err != nil {
	//	sklog.Fatal("Could not get from firestore contents of %s: %s", col, err)
	//}
	//fmt.Println("GOT THIS OUT!!!")
	//fmt.Printf("%+v", fromB)

	/*

			        c.Timestamp = firestore.FixTimestamp(c.Timestamp)
		        id := taskCommentId(c)
		        _, err := d.client.Create(context.TODO(), d.taskComments().Doc(id), c, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT)
		        if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
		                return db.ErrAlreadyExists
		        }
		        if err != nil {
		                return err
		        }
		        return nil

	*/

	//pubSubReceive := metrics2.NewLiveness("ci_pubsub_receive_v2", nil)
	for {
		err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			fmt.Println("GOT SOMETHING!!!!!!")
			// Check here to see what the shortImage name is.
			// If it is skia-release-v2 or skia-wasm-release-v2 or skia-infra-v2 then do the prod thingy.
			// For everything do the pushk thingy.

			msg.Ack()
			sklog.Infof("Status: %s", msg.Attributes["status"])

			var buildInfo continuous_deploy.BuildInfo
			if err := json.Unmarshal(msg.Data, &buildInfo); err != nil {
				sklog.Errorf("Failed to decode: %s: %q", err, string(msg.Data))
				return
			}
			//repoName := "--unknown--"
			//if buildInfo.Source != nil && buildInfo.Source.RepoSource != nil {
			//	repoName = buildInfo.Source.RepoSource.RepoName
			//}

			// No need because it will be a bot visible on the tree.
			//// Record build failures so we can alert on them.
			//counter := metrics2.GetCounter("ci_build_failure_v2", map[string]string{"image_name": buildInfo.ImageName})
			//if msg.Attributes["status"] == "FAILURE" {
			//	counter.Inc(1)
			//} else if msg.Attributes["status"] == "SUCCESS" {
			//	counter.Reset()
			//}

			//if msg.Attributes["status"] != "SUCCESS" {
			//	return
			//}
			imageName, tag := imageAndTagFromInfo(*deployImages, buildInfo)
			if err != nil {
				sklog.Error(err)
				return
			}
			if imageName == "" {
				sklog.Infof("No image to push.")
				return
			}
			// Commit tags contain the commit hash. Trybot tags are are this format: ${CHANGE_NUM}/${PATCHSET_NUM}.
			// Ignore trybot tags and only apply prod tag to commit hashes.
			if strings.Index(tag, "_") != -1 {
				sklog.Infof("Found a trybot tag %s. Ignoring.", tag)
				return
			}

			// Do firestore to compare with what we already encountered. Put a mutex around this.

			// UNCOMMENT THE BELOW TO ACTUALLY PUSH TO SOMETHING!!!!!!
			//cmd := fmt.Sprintf("%s --logtostderr %s", pushk, strings.Join(imageNames, " "))
			//sklog.Infof("About to execute: %q", cmd)
			//output, err := exec.RunSimple(ctx, cmd)
			//pushFailure := metrics2.GetCounter("ci_push_failure_v2", map[string]string{"image_name": buildInfo.ImageName})
			//if err != nil {
			//	sklog.Errorf("Failed to run pushk: %s: %s", output, err)
			//	pushFailure.Inc(1)
			//	return
			//} else {
			//	sklog.Info(output)
			//}
			//pushFailure.Reset()
			//pubSubReceive.Reset()
			sklog.Info("Finished push")

			fmt.Println("DONE DONE DONE")
		})
		if err != nil {
			sklog.Errorf("Failed receiving pubsub message: %s", err)
		}
	}
}
