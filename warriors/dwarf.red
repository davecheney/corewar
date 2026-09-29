;redcode-94
;name Dwarf
;author A. K. Dewdney
; Drops a DAT bomb on every fourth cell of the core.
        ORG     loop
loop    ADD.AB  #4, bomb
        MOV.I   bomb, @bomb
        JMP     loop
bomb    DAT     #0, #0
