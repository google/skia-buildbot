package config

const (
	// Different datasets that are stored in tiles.
	DATASET_GOLD = "gold"

	// Constructor names that are used to instantiate an ingester.
	// Note that, e.g. 'android-gold' has a different ingester, but writes
	// to the gold dataset.
	CONSTRUCTOR_GOLD         = DATASET_GOLD
	CONSTRUCTOR_ANDROID_GOLD = "android-gold"
)
