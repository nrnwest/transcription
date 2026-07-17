Output "assets/demo.gif"
Set Shell "bash"
Set FontSize 20
Set Width 1200
Set Height 720
Set Padding 24
Set Theme "Dracula"
Set TypingSpeed 55ms

Hide
Type "export PS1='$ '" Enter
Type "cd /tmp/tr-demo && rm -f example/speakers-3/speakers-3.md && clear" Enter
Show

Sleep 600ms
Type "ls example/speakers-3/" Enter
Sleep 1500ms
Type "./transcription-run example/speakers-3/" Enter
Sleep 3000ms
Screenshot "assets/progress.png"
Sleep 900ms
Type "cat example/speakers-3/speakers-3.md" Enter
Sleep 3500ms
