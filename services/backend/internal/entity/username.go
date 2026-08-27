package entity

import (
	"math/rand/v2"
	"strings"
)

// The three pools a generated username is drawn from: two descriptive words followed by an animal,
// giving names like "sly_dancing_monkey". Every word is lowercase ASCII and at most nine characters,
// so any combination is at most 29 characters and satisfies SetUsername - TestNewUsername pins both.
var (
	usernameAdjectives = []string{
		"amber", "bold", "brave", "breezy", "calm", "cheery", "chirpy", "clever",
		"cosy", "dapper", "dusty", "eager", "feisty", "fierce", "frosty", "gentle",
		"glossy", "golden", "hazy", "humble", "jaunty", "jolly", "keen", "lively",
		"lucky", "mellow", "merry", "mild", "nimble", "noble", "peppy", "plucky",
		"proud", "quiet", "quirky", "rapid", "rosy", "rustic", "shy", "silent",
		"sleepy", "sly", "smooth", "snug", "sparkly", "spry", "stormy", "sturdy",
		"sunlit", "sunny", "swift", "tawny", "tender", "tidy", "upbeat", "velvet",
		"vivid", "wily", "wistful", "witty", "woolly", "zany", "zesty", "zippy",
	}
	usernameActions = []string{
		"bouncing", "bounding", "chirping", "climbing", "coasting", "dancing",
		"dashing", "doodling", "dreaming", "drifting", "floating", "giggling",
		"gliding", "grinning", "hopping", "howling", "humming", "jumping",
		"laughing", "leaping", "marching", "napping", "nodding", "paddling",
		"padding", "playing", "pouncing", "prancing", "reading", "roaming",
		"running", "sailing", "shuffling", "sighing", "singing", "sketching",
		"skipping", "sleeping", "smiling", "sneaking", "snoozing", "soaring",
		"spinning", "sprinting", "strutting", "swimming", "trotting", "tumbling",
		"twirling", "waddling", "waltzing", "wandering", "whirling", "yawning",
		"zooming",
	}
	usernameAnimals = []string{
		"alpaca", "antelope", "armadillo", "axolotl", "badger", "beaver", "bison",
		"buffalo", "caribou", "cheetah", "chipmunk", "dingo", "dormouse", "egret",
		"falcon", "ferret", "gazelle", "gecko", "gibbon", "giraffe", "gopher",
		"grouse", "hamster", "hedgehog", "heron", "ibex", "iguana", "impala",
		"jackal", "jackdaw", "jaguar", "kangaroo", "kestrel", "koala", "lemur",
		"lizard", "lynx", "magpie", "mallard", "marten", "meerkat", "mongoose",
		"moose", "muskrat", "narwhal", "newt", "numbat", "ocelot", "opossum",
		"osprey", "ostrich", "otter", "panda", "pangolin", "parakeet", "peacock",
		"pelican", "pheasant", "platypus", "polecat", "porpoise", "puffin",
		"python", "quail", "quokka", "rabbit", "raccoon", "raven", "reindeer",
		"salmon", "seal", "sparrow", "squirrel", "starling", "stoat", "swallow",
		"tapir", "tortoise", "toucan", "turtle", "urchin", "vulture", "wallaby",
		"walrus", "warbler", "weasel", "wolverine", "wombat", "yak", "zebra",
	}
)

// NewUsername returns a random three-word username, e.g. "sly_dancing_monkey". It is what every
// profile is named at sign-up, since callers are not asked to pick a name for themselves.
//
// The pools multiply out to hundreds of thousands of names, which makes a collision unlikely rather
// than impossible - so a caller assigning one draws again on repository.ErrUsernameTaken instead of
// trusting a single draw. Checking a name for availability first would be both slower and racy, as
// only the write itself can claim it.
func NewUsername() string {
	return strings.Join([]string{
		usernameAdjectives[rand.IntN(len(usernameAdjectives))],
		usernameActions[rand.IntN(len(usernameActions))],
		usernameAnimals[rand.IntN(len(usernameAnimals))],
	}, "_")
}
