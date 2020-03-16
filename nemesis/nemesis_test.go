package nemesis

import (
	"fmt"
	"testing"

	"github.com/deatheyes/chaos/common"
)

func TestCreateEmpty(t *testing.T) {
	fmt.Println("TestCreateEmpty")
	task := &Task{}
	_, err := NewManager().CreateNemesis(task)
	if err != ErrUnknown {
		t.Errorf("unexpected error %v", err)
	}
}

func TestCreatePartation(t *testing.T) {
	fmt.Println("TestCreatePartation")
	task := &Task{
		Type:      common.TaskPartition,
		Arguments: make(map[string]string),
	}
	task.Arguments[common.ArgumentsDebug] = "true"
	task.Arguments[common.ArgumentsKeyNodes] = "127.0.0.1"
	m := NewManager()
	id, err := m.CreateNemesis(task)
	if err != nil {
		t.Errorf("create partition failed: %v", err)
	}

	if _, ok := m.NemesisDict[id]; !ok {
		t.Errorf("nemesis not exist")
	}

	if _, ok := m.NemesisDict[id].(*Partition); !ok {
		t.Errorf("unexpected type: %v", m.NemesisDict[id].GetType())
	}
}

func TestCreatePartationWithoutNodes(t *testing.T) {
	fmt.Println("TestCreatePartationWithoutNodes")
	task := &Task{
		Type:      common.TaskPartition,
		Arguments: make(map[string]string),
	}
	task.Arguments[common.ArgumentsDebug] = "true"
	m := NewManager()
	_, err := m.CreateNemesis(task)
	if err != ErrEmptyNodes {
		t.Errorf("create partition failed: %v", err)
	}
}

func TestCreateKill(t *testing.T) {
	fmt.Println("TestCreateKill")
	task := &Task{
		Type:      common.TaskService,
		Arguments: make(map[string]string),
	}
	task.Arguments[common.ArgumentsKeyRun] = "echo run"
	task.Arguments[common.ArgumentsKeyTerminate] = "echo terminate"
	m := NewManager()
	id, err := m.CreateNemesis(task)
	if err != nil {
		t.Errorf("create partition failed: %v", err)
	}

	if _, ok := m.NemesisDict[id]; !ok {
		t.Errorf("nemesis not exist")
	}

	if _, ok := m.NemesisDict[id].(*Service); !ok {
		t.Errorf("unexpected type: %v", m.NemesisDict[id].GetType())
	}
}

func TestNotFound(t *testing.T) {
	fmt.Println("TestNotFound")
	err := NewManager().TerminateNemesis(&Task{})
	if err != ErrNotFount {
		t.Errorf("unexpected error: %v", err)
	}
}
