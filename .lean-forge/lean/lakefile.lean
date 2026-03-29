import Lake
open Lake DSL

package «spec» where
  leanOptions := #[
    ⟨`autoImplicit, false⟩
  ]

@[default_target]
lean_lib «Spec» where
  srcDir := "."
