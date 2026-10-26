namespace Orge.Store
{
    /// <summary>
    /// Holds each class of region; a Flush(Region r) call writes one.
    /// </summary>
    public class Store
    {
        public async Task<int> Flush(Region region,
                                     CancellationToken token)
        {
            return await region.Write(token);
        }

        /* public void Hidden(int x)
           {
           } */
        public void Clear() => Reset();
    }
}
