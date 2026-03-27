
package model

type BeamlineID string

// Dataset DID example:
// /beamline=id3a/btr=val123/cycle=123/sample_name=bla
type DatasetDID string

type DatasetRef struct {
    Beamline BeamlineID
    DID      DatasetDID
}
