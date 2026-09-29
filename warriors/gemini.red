;redcode-94
;name Gemini
;author after A. K. Dewdney
; Copies itself 100 cells ahead, jumps into the copy and does it again.
src     DAT     #0, #0
dst     DAT     #0, #99
start   MOV.I   @src, @dst
        ADD.AB  #1, src
        ADD.AB  #1, dst
        SEQ.B   src, #last-src+1
        JMP     start
        MOV.AB  #99, dst+100
last    JMP     start+100
        END     start
