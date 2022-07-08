/*
 * Copyright (C) 2019 ~ 2022 Uniontech Software Technology Co.,Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */package scheduler

type CbId struct {
	Idx uint32
	Val uint32
}

type CnMsg struct {
	Id   CbId
	Seq  uint32
	Ack  uint32
	Len  uint16
	Flag uint16
}

// ProcEventHeader corresponds to proc_event in cn_proc.h
type ProcEventHeader struct {
	What      uint32
	CPU       uint32
	Timestamp uint64
}

// fork proc event
type ForkProcEvent struct {
	ParentPid  uint32
	ParentTGid uint32
	ChildPid   uint32
	ChildTGid  uint32
}

// exec proc event
type ExecProcEvent struct {
	ProcPid  uint32
	ProcTGid uint32
}

// id proc event
type IdProcEvent struct {
	ProcPid  uint32
	ProcTGid uint32
}

// exit proc event
type ExitProcEvent struct {
	ProcessPid  uint32
	ProcessTgid uint32
	ExitCode    uint32
	ExitSignal  uint32
}

// comm event
type CommEvent struct {
	ProcessPid  uint32
	ProcessTgid uint32
	Buf         [16]byte
}
