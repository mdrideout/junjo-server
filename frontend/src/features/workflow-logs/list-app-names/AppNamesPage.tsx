import { Link } from 'react-router'
import AppNamesList from './AppNamesList'

/**
 * App Names Page
 *
 * Lists the unique app names in the database.
 * @returns
 */
export default function AppNamesPage() {
  // Human readable start ingest time
  const date = new Date()
  const readableDate = date.toLocaleString()

  return (
    <div className={'p-5'}>
      <div className={'px-2'}>
        <div className={'mb-1 flex gap-x-3 font-bold'}>
          <Link to={'/logs'} className={'hover:underline'}>
            Logs
          </Link>{' '}
        </div>
        <div className={'text-zinc-400 text-xs'}>{readableDate}</div>
      </div>
      <hr className={'my-6'} />
      <AppNamesList />
    </div>
  )
}
