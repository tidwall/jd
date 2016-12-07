package gjson

import (
	"encoding/json"
	"fmt"
	"testing"
)

type Person struct {
	ID   string `json:"id"`
	Name struct {
		First string `json:"first"`
		Last  string `json:"last"`
	} `json:"name"`
	Age     int      `json:"age"`
	Friends []string `json:"friends"`
}

const jsonStr = `
{
  "id": "b39d3eaf813a",
  "name": {
    "first": "Randi",
    "last": "Andrews"
  },
  "age": 29,
  "friends": [
    "Bill", "Sharell", "Karen", "Tom"
  ]
}
`

func TestUnmarshal(t *testing.T) {
	var person Person
	res := GetMany(jsonStr, "id", "name.first", "name.last", "age", "friends")
	person.ID = res[0].String()
	person.Name.First = res[1].String()
	person.Name.Last = res[2].String()
	person.Age = int(res[3].Int())
	res[4].ForEach(func(key, val Result) bool {
		person.Friends = append(person.Friends, val.String())
		return true
	})
	fmt.Printf("%v\n", person)
}

func BenchmarkPersonUnmarshalGJSON(t *testing.B) {
	var person Person
	person.Friends = make([]string, 8)
	t.ReportAllocs()
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		res := GetMany(jsonStr, "id", "name.first", "name.last", "age", "friends")
		person.ID = res[0].String()
		person.Name.First = res[1].String()
		person.Name.Last = res[2].String()
		person.Age = int(res[3].Int())
		person.Friends = person.Friends[:0]
		res[4].ForEach(func(key, val Result) bool {
			person.Friends = append(person.Friends, val.String())
			return true
		})
	}
}

func BenchmarkPersonUnmarshalGoJSON(t *testing.B) {
	jsonBytes := []byte(jsonStr)
	var person Person
	person.Friends = make([]string, 8)
	t.ReportAllocs()
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		json.Unmarshal(jsonBytes, &person)
	}
}
