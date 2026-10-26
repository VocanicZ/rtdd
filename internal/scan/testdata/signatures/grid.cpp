#include "grid.h"

/*
 * int phantom(int x)
 * class Ghost {
 */
static int cell_index(int x,
                      int y,
                      int z)
{
    return x + 16 * (y + 16 * z);
}

// struct Ghost { int step(int dt) { return dt; } };
int Grid::step(const World& world,
               double dt) const {
    return cell_index(0, 0, 0);
}
