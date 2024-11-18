// Code generated by https://github.com/gagliardetto/anchor-go. DO NOT EDIT.

package squads_mpl

import (
	"errors"
	ag_binary "github.com/gagliardetto/binary"
	ag_solanago "github.com/gagliardetto/solana-go"
	ag_format "github.com/gagliardetto/solana-go/text/format"
	ag_treeout "github.com/gagliardetto/treeout"
)

// The instruction to remove a member from the multisig
type RemoveMember struct {
	OldMember *ag_solanago.PublicKey

	// [0] = [WRITE, SIGNER] multisig
	ag_solanago.AccountMetaSlice `bin:"-"`
}

// NewRemoveMemberInstructionBuilder creates a new `RemoveMember` instruction builder.
func NewRemoveMemberInstructionBuilder() *RemoveMember {
	nd := &RemoveMember{
		AccountMetaSlice: make(ag_solanago.AccountMetaSlice, 1),
	}
	return nd
}

// SetOldMember sets the "oldMember" parameter.
func (inst *RemoveMember) SetOldMember(oldMember ag_solanago.PublicKey) *RemoveMember {
	inst.OldMember = &oldMember
	return inst
}

// SetMultisigAccount sets the "multisig" account.
func (inst *RemoveMember) SetMultisigAccount(multisig ag_solanago.PublicKey) *RemoveMember {
	inst.AccountMetaSlice[0] = ag_solanago.Meta(multisig).WRITE().SIGNER()
	return inst
}

// GetMultisigAccount gets the "multisig" account.
func (inst *RemoveMember) GetMultisigAccount() *ag_solanago.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

func (inst RemoveMember) Build() *Instruction {
	return &Instruction{BaseVariant: ag_binary.BaseVariant{
		Impl:   inst,
		TypeID: Instruction_RemoveMember,
	}}
}

// ValidateAndBuild validates the instruction parameters and accounts;
// if there is a validation error, it returns the error.
// Otherwise, it builds and returns the instruction.
func (inst RemoveMember) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *RemoveMember) Validate() error {
	// Check whether all (required) parameters are set:
	{
		if inst.OldMember == nil {
			return errors.New("OldMember parameter is not set")
		}
	}

	// Check whether all (required) accounts are set:
	{
		if inst.AccountMetaSlice[0] == nil {
			return errors.New("accounts.Multisig is not set")
		}
	}
	return nil
}

func (inst *RemoveMember) EncodeToTree(parent ag_treeout.Branches) {
	parent.Child(ag_format.Program(ProgramName, ProgramID)).
		//
		ParentFunc(func(programBranch ag_treeout.Branches) {
			programBranch.Child(ag_format.Instruction("RemoveMember")).
				//
				ParentFunc(func(instructionBranch ag_treeout.Branches) {

					// Parameters of the instruction:
					instructionBranch.Child("Params[len=1]").ParentFunc(func(paramsBranch ag_treeout.Branches) {
						paramsBranch.Child(ag_format.Param("OldMember", *inst.OldMember))
					})

					// Accounts of the instruction:
					instructionBranch.Child("Accounts[len=1]").ParentFunc(func(accountsBranch ag_treeout.Branches) {
						accountsBranch.Child(ag_format.Meta("multisig", inst.AccountMetaSlice.Get(0)))
					})
				})
		})
}

func (obj RemoveMember) MarshalWithEncoder(encoder *ag_binary.Encoder) (err error) {
	// Serialize `OldMember` param:
	err = encoder.Encode(obj.OldMember)
	if err != nil {
		return err
	}
	return nil
}
func (obj *RemoveMember) UnmarshalWithDecoder(decoder *ag_binary.Decoder) (err error) {
	// Deserialize `OldMember`:
	err = decoder.Decode(&obj.OldMember)
	if err != nil {
		return err
	}
	return nil
}

// NewRemoveMemberInstruction declares a new RemoveMember instruction with the provided parameters and accounts.
func NewRemoveMemberInstruction(
	// Parameters:
	oldMember ag_solanago.PublicKey,
	// Accounts:
	multisig ag_solanago.PublicKey) *RemoveMember {
	return NewRemoveMemberInstructionBuilder().
		SetOldMember(oldMember).
		SetMultisigAccount(multisig)
}
