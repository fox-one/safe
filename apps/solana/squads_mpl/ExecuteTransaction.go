// Code generated by https://github.com/gagliardetto/anchor-go. DO NOT EDIT.

package squads_mpl

import (
	"errors"
	ag_binary "github.com/gagliardetto/binary"
	ag_solanago "github.com/gagliardetto/solana-go"
	ag_format "github.com/gagliardetto/solana-go/text/format"
	ag_treeout "github.com/gagliardetto/treeout"
)

// Instruction to execute a transaction.
// Transaction status must be "executeReady", and the account list must match
// the unique indexed accounts in the following manner:
// [ix_1_account, ix_1_program_account, ix_1_remaining_account_1, ix_1_remaining_account_2, ...]
//
// Refer to the README for more information on how to construct the account list.
type ExecuteTransaction struct {
	AccountList *[]byte

	// [0] = [WRITE] multisig
	//
	// [1] = [WRITE] transaction
	//
	// [2] = [WRITE, SIGNER] member
	ag_solanago.AccountMetaSlice `bin:"-"`
}

// NewExecuteTransactionInstructionBuilder creates a new `ExecuteTransaction` instruction builder.
func NewExecuteTransactionInstructionBuilder() *ExecuteTransaction {
	nd := &ExecuteTransaction{
		AccountMetaSlice: make(ag_solanago.AccountMetaSlice, 3),
	}
	return nd
}

// SetAccountList sets the "accountList" parameter.
func (inst *ExecuteTransaction) SetAccountList(accountList []byte) *ExecuteTransaction {
	inst.AccountList = &accountList
	return inst
}

// SetMultisigAccount sets the "multisig" account.
func (inst *ExecuteTransaction) SetMultisigAccount(multisig ag_solanago.PublicKey) *ExecuteTransaction {
	inst.AccountMetaSlice[0] = ag_solanago.Meta(multisig).WRITE()
	return inst
}

// GetMultisigAccount gets the "multisig" account.
func (inst *ExecuteTransaction) GetMultisigAccount() *ag_solanago.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

// SetTransactionAccount sets the "transaction" account.
func (inst *ExecuteTransaction) SetTransactionAccount(transaction ag_solanago.PublicKey) *ExecuteTransaction {
	inst.AccountMetaSlice[1] = ag_solanago.Meta(transaction).WRITE()
	return inst
}

// GetTransactionAccount gets the "transaction" account.
func (inst *ExecuteTransaction) GetTransactionAccount() *ag_solanago.AccountMeta {
	return inst.AccountMetaSlice.Get(1)
}

// SetMemberAccount sets the "member" account.
func (inst *ExecuteTransaction) SetMemberAccount(member ag_solanago.PublicKey) *ExecuteTransaction {
	inst.AccountMetaSlice[2] = ag_solanago.Meta(member).WRITE().SIGNER()
	return inst
}

// GetMemberAccount gets the "member" account.
func (inst *ExecuteTransaction) GetMemberAccount() *ag_solanago.AccountMeta {
	return inst.AccountMetaSlice.Get(2)
}

func (inst ExecuteTransaction) Build() *Instruction {
	return &Instruction{BaseVariant: ag_binary.BaseVariant{
		Impl:   inst,
		TypeID: Instruction_ExecuteTransaction,
	}}
}

// ValidateAndBuild validates the instruction parameters and accounts;
// if there is a validation error, it returns the error.
// Otherwise, it builds and returns the instruction.
func (inst ExecuteTransaction) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *ExecuteTransaction) Validate() error {
	// Check whether all (required) parameters are set:
	{
		if inst.AccountList == nil {
			return errors.New("AccountList parameter is not set")
		}
	}

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
	}
	return nil
}

func (inst *ExecuteTransaction) EncodeToTree(parent ag_treeout.Branches) {
	parent.Child(ag_format.Program(ProgramName, ProgramID)).
		//
		ParentFunc(func(programBranch ag_treeout.Branches) {
			programBranch.Child(ag_format.Instruction("ExecuteTransaction")).
				//
				ParentFunc(func(instructionBranch ag_treeout.Branches) {

					// Parameters of the instruction:
					instructionBranch.Child("Params[len=1]").ParentFunc(func(paramsBranch ag_treeout.Branches) {
						paramsBranch.Child(ag_format.Param("AccountList", *inst.AccountList))
					})

					// Accounts of the instruction:
					instructionBranch.Child("Accounts[len=3]").ParentFunc(func(accountsBranch ag_treeout.Branches) {
						accountsBranch.Child(ag_format.Meta("   multisig", inst.AccountMetaSlice.Get(0)))
						accountsBranch.Child(ag_format.Meta("transaction", inst.AccountMetaSlice.Get(1)))
						accountsBranch.Child(ag_format.Meta("     member", inst.AccountMetaSlice.Get(2)))
					})
				})
		})
}

func (obj ExecuteTransaction) MarshalWithEncoder(encoder *ag_binary.Encoder) (err error) {
	// Serialize `AccountList` param:
	err = encoder.Encode(obj.AccountList)
	if err != nil {
		return err
	}
	return nil
}
func (obj *ExecuteTransaction) UnmarshalWithDecoder(decoder *ag_binary.Decoder) (err error) {
	// Deserialize `AccountList`:
	err = decoder.Decode(&obj.AccountList)
	if err != nil {
		return err
	}
	return nil
}

// NewExecuteTransactionInstruction declares a new ExecuteTransaction instruction with the provided parameters and accounts.
func NewExecuteTransactionInstruction(
	// Parameters:
	accountList []byte,
	// Accounts:
	multisig ag_solanago.PublicKey,
	transaction ag_solanago.PublicKey,
	member ag_solanago.PublicKey) *ExecuteTransaction {
	return NewExecuteTransactionInstructionBuilder().
		SetAccountList(accountList).
		SetMultisigAccount(multisig).
		SetTransactionAccount(transaction).
		SetMemberAccount(member)
}