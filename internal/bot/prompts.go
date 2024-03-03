package bot

import (
	"math/rand"
	"strings"

	"github.com/AlexGustafsson/clabbe/internal/openai"
)

func NewThemeRequest() *openai.CompletionRequest {
	prompt := `Respond with themes for songs to include in a playlist. Respond with five themes, one theme per line, each containing at least four words. Don't include seasonal themes such as summer vibes or Christmas songs.

Don't include the following words or similar words:
Workout
Empowerment
Feel-good
Study
Love`

	examples := strings.Split(`Underground indie rock vibes
Classic rock ballads revisited
West coast hip-hop classics
Latin jazz fusion grooves
Acoustic soulful folk tunes
Underground indie rock vibes
Classic rock ballads revisited
West coast hip-hop classics
Latin jazz fusion grooves
Acoustic soulful folk tunes
Synthwave retro cyber beats
Reggae essentials for relaxation
Indie pop anthems for road trips
90s R&B slow jams sentiment
High-energy EDM bangers
Chill lo-fi hip-hop vibes
Alternative rock anthems mix
Country music storytelling tunes
Jazzy swing dance classics`, "\n")

	// Pick a few examples to include for context, providing a bit more random
	// state to the model, which is otherwise quite deterministic
	selectedExamples := make([]string, 5)
	for i := 0; i < 5; i++ {
		selectedExamples[i] = examples[rand.Intn(len(examples))]
	}

	return &openai.CompletionRequest{
		Messages: []openai.Message{
			{
				Role:    openai.RoleSystem,
				Content: prompt,
			},
			{
				Role:    openai.RoleAssistant,
				Content: strings.Join(selectedExamples, "\n"),
			},
		},
		Temperature: 1.2,
		// 50*4(average token length)=200. 5 examples are typically 100 characters.
		MaxTokens:        50,
		TopP:             1,
		FrequencyPenalty: 0.2,
		PresencePenalty:  0.2,
		Model:            openai.DefaultModel,
		Stream:           false,
	}
}
