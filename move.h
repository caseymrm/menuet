#ifndef __MOVE_H_H__
#define __MOVE_H_H__

#include <stdbool.h>

char *menuetOriginalBundlePath(const char *path, bool *translocated);
int menuetMoveAlert(const char *message, const char *info,
                    const char *firstButton, const char *secondButton,
                    bool showSuppression, bool *suppressed);
bool menuetOtherInstanceRunningAt(const char *bundleID, const char *path);
char *menuetTrash(const char *path);

#endif
