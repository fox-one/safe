// Code generated by https://github.com/gagliardetto/anchor-go. DO NOT EDIT.

package squads_mpl

import (
	"errors"
	ag_binary "github.com/gagliardetto/binary"
	ag_solanago "github.com/gagliardetto/solana-go"
	ag_format "github.com/gagliardetto/solana-go/text/format"
	ag_treeout "github.com/gagliardetto/treeout"
)

// Instruction to cancel a transaction.
// Transactions must be in the "executeReady" status.
// Transaction will only be cancelled if the number of
// cancellations reaches the threshold. A cancelled
// transaction will no longer be able to be executed.
type CancelTransaction struct {

	// [0] = [WRITE] multisig
	//
	// [1] = [WRITE] transaction
	//
	// [2] = [WRITE, SIGNER] member
	//
	// [3] = [] systemProgram
	ag_solanago.AccountMetaSlice `bin:"-"`
}

// NewCancelTransactionInstructionBuilder creates a new `CancelTransaction` instruction builder.
func NewCancelTransactionInstructionBuilder() *CancelTransaction {
	nd := &CancelTransaction{
		AccountMetaSlice: make(ag_solanago.AccountMetaSlice, 4),
	}
	return nd
}

// SetMultisigAccount sets the "multisig" account.
func (inst *CancelTransaction) SetMultisigAccount(multisig ag_solanago.PublicKey) *CancelTransaction {
	inst.AccountMetaSlice[0] = ag_solanago.Meta(multisig).WRITE()
	return inst
}

// GetMultisigAccount gets the "multisig" account.
func (inst *CancelTransaction) GetMultisigAccount() *ag_solanago.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

// SetTransactionAccount sets the "transaction" account.
func (inst *CancelTransaction) SetTransactionAccount(transaction ag_solanago.PublicKey) *CancelTransaction {
	inst.AccountMetaSlice[1] = ag_solanago.Meta(transaction).WRITE()
	return inst
}

// GetTransactionAccount gets the "transaction" account.
func (inst *CancelTransaction) GetTransactionAccount() *ag_solanago.AccountMeta {
	return inst.AccountMetaSlice.Get(1)
}

// SetMemberAccount sets the "member" account.
func (inst *CancelTransaction) SetMemberAccount(member ag_solanago.PublicKey) *CancelTransaction {
	inst.AccountMetaSlice[2] = ag_solanago.Meta(member).WRITE().SIGNER()
	return inst
}

// GetMemberAccount gets the "member" account.
func (inst *CancelTransaction) GetMemberAccount() *ag_solanago.AccountMeta {
	return inst.AccountMetaSlice.Get(2)
}

// SetSystemProgramAccount sets the "systemProgram" account.
func (inst *CancelTransaction) SetSystemProgramAccount(systemProgram ag_solanago.PublicKey) *CancelTransaction {
	inst.AccountMetaSlice[3] = ag_solanago.Meta(systemProgram)
	return inst
}

// GetSystemProgramAccount gets the "systemProgram" account.
func (inst *CancelTransaction) GetSystemProgramAccount() *ag_solanago.AccountMeta {
	return inst.AccountMetaSlice.Get(3)
}

func (inst CancelTransaction) Build() *Instruction {
	return &Instruction{BaseVariant: ag_binary.BaseVariant{
		Impl:   inst,
		TypeID: Instruction_CancelTransaction,
	}}
}

// ValidateAndBuild validates the instruction parameters and accounts;
// if there is a validation error, it returns the error.
// Otherwise, it builds and returns the instruction.
func (inst CancelTransaction) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *CancelTransaction) Validate() error {
	// Check whether all (required) accounts are set:
	{
		if inst.AccountMetaSlice[0] == nil {
			return errors.New("accounts.Multisig is not set")
		}
		if inst.AccountMetaSlice[1] == nil {
			return errors.New("accounts.Transaction is not set")
		}
		if inst.AccountMetaSlice[2] == nil {
			return errors.New("accounts.Member is not set")
		}
		if inst.AccountMetaSlice[3] == nil {
			return errors.New("accounts.SystemProgram is not set")
		}
	}
	return nil
}

func (inst *CancelTransaction) EncodeToTree(parent ag_treeout.Branches) {
	parent.Child(ag_format.Program(ProgramName, ProgramID)).
		//
		ParentFunc(func(programBranch ag_treeout.Branches) {
			programBranch.Child(ag_format.Instruction("CancelTransaction")).
				//
				ParentFunc(func(instructionBranch ag_treeout.Branches) {

					// Parameters of the instruction:
					instructionBranch.Child("Params[len=0]").ParentFunc(func(paramsBranch ag_treeout.Branches) {})

					// Accounts of the instruction:
					instructionBranch.Child("Accounts[len=4]").ParentFunc(func(accountsBranch ag_treeout.Branches) {
						accountsBranch.Child(ag_format.Meta("     multisig", inst.AccountMetaSlice.Get(0)))
						accountsBranch.Child(ag_format.Meta("  transaction", inst.AccountMetaSlice.Get(1)))
						accountsBranch.Child(ag_format.Meta("       member", inst.AccountMetaSlice.Get(2)))
						accountsBranch.Child(ag_format.Meta("systemProgram", inst.AccountMetaSlice.Get(3)))
					})
				})
		})
}

func (obj CancelTransaction) MarshalWithEncoder(encoder *ag_binary.Encoder) (err error) {
	return nil
}
func (obj *CancelTransaction) UnmarshalWithDecoder(decoder *ag_binary.Decoder) (err error) {
	return nil
}

// NewCancelTransactionInstruction declares a new CancelTransaction instruction with the provided parameters and accounts.
func NewCancelTransactionInstruction(
	// Accounts:
	multisig ag_solanago.PublicKey,
	transaction ag_solanago.PublicKey,
	member ag_solanago.PublicKey,
	systemProgram ag_solanago.PublicKey) *CancelTransaction {
	return NewCancelTransactionInstructionBuilder().
		SetMultisigAccount(multisig).
		SetTransactionAccount(transaction).
		SetMemberAccount(member).
		SetSystemProgramAccount(systemProgram)
}
