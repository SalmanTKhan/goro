package game

import (
	"testing"

	"github.com/kivutar/goro/db"
)

func TestInferLegacyActorObjectType(t *testing.T) {
	tests := []struct {
		name string
		job  int16
		want uint8
	}{
		{name: "player", job: db.JobNovice, want: actorObjectTypePC},
		{name: "monster", job: 1002, want: actorObjectTypeMob},
		{name: "npc", job: 45, want: actorObjectTypeNPC},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := inferLegacyActorObjectType(test.job)
			if !ok || got != test.want {
				t.Fatalf("inferLegacyActorObjectType(%d) = %d, %t; want %d, true", test.job, got, ok, test.want)
			}
		})
	}
}
