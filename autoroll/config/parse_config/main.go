package main

func main() {
    config = &config.Config{
	RollerName: "skia-autoroll",
	ChildDisplayName: "Skia",
	ParentDisplayName: "Chromium",
	OwnerPrimary: "borenet",
    OwnerSecondary: "rmistry",
	Contacts: []string{"borenet@google.com"},
	ServiceAccount: "chromium-autoroll@skia-public.iam.gserviceaccount.com",
	IsInternal: false,
	Sheriff: []string{"https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener"},
	SupportsManualRolls: true,
	CommitMsg &config.CommitMsgConfig{},
	CodeReview isConfig_CodeReview `protobuf_oneof:"code_review"`
	Kubernetes *KubernetesConfig `protobuf:"bytes,18,opt,name=kubernetes,proto3" json:"kubernetes,omitempty"`
	RepoManager isConfig_RepoManager `protobuf_oneof:"repo_manager"`
	Notifiers []*NotifierConfig `protobuf:"bytes,25,rep,name=notifiers,proto3" json:"notifiers,omitempty"`
    }
}